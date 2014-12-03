package webterm

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/influxdb"

	"github.com/coreos/go-etcd/etcd"
	"github.com/quipo/statsd"
)

type WorkClient struct {
	Config             *AppConfiguration
	MarshalledConfig   string
	quit               chan struct{}
	logclosed          chan struct{}
	runtimeclosed      chan struct{}
	heartbeatclosed    chan struct{}
	reconnectclosed    chan struct{}
	runtimeTicker      *time.Ticker
	heartbeatTicker    *time.Ticker
	events             chan LogEvent
	statsdClient       *statsd.StatsdClient
	etcdClient         *etcd.Client
	stats              *statsd.StatsdBuffer
	gaugeMap           map[string]metrics.Gauge
	timeMap            map[string]metrics.Timer
	executeWorkFn      func()
	closeExecuteWorkFn func()
}

// Configure the client according to the given configuration and initialize state
func (client *WorkClient) Configure(config *AppConfiguration, executeWorkFn, closeExecuteWorkFn func()) {
	client.logclosed = make(chan struct{}, 1)
	client.runtimeclosed = make(chan struct{}, 1)
	client.heartbeatclosed = make(chan struct{}, 1)
	client.reconnectclosed = make(chan struct{}, 1)
	client.quit = make(chan struct{}, 1)
	client.runtimeTicker = time.NewTicker(time.Second)
	client.heartbeatTicker = time.NewTicker(time.Second)
	client.events = make(chan LogEvent, 10)
	client.executeWorkFn = executeWorkFn
	client.closeExecuteWorkFn = closeExecuteWorkFn
	client.gaugeMap = make(map[string]metrics.Gauge)
	client.timeMap = make(map[string]metrics.Timer)
	client.Config = config

	if client.Config.Hostname == "" {
		if len(os.Getenv("HOSTNAME")) != 0 {
			client.Config.Hostname = os.Getenv("HOSTNAME")
		} else {
			h, err := os.Hostname()
			if err != nil {
				panic(err)
			}
			client.Config.Hostname = h
		}
	}
}

// Runs all goroutines for signals, closing, connect routines..etc
func (c *WorkClient) Run() {
	defer func() {
		<-c.quit
		os.Exit(0)
	}()

	// Ensure that the stats client is closed on close
	if c.Config.StatsdAddr != "" {
		c.statsdClient = statsd.NewStatsdClient(c.Config.StatsdAddr, c.Config.StatsdPrefix)
		c.statsdClient.CreateSocket()
		interval := time.Second * time.Duration(c.Config.StatsdInterval)
		c.stats = statsd.NewStatsdBuffer(interval, c.statsdClient)

		defer c.stats.Close()
	}
	go c.writeRuntimeStats()

	// Register with etcd for service configuration/discovery/heartbeat monitor
	if c.Config.EtcdAddr != "" {
		c.heartbeatTicker = time.NewTicker(time.Second * time.Duration(c.Config.EtcdHeartbeatTtl))
		if c.Config.EtcdCaCert != "" && c.Config.EtcdTlsKey != "" && c.Config.EtcdTlsCert != "" {
			etcdClient, err := etcd.NewTLSClient([]string{c.Config.EtcdAddr}, c.Config.EtcdTlsCert, c.Config.EtcdTlsKey, c.Config.EtcdCaCert)
			if err != nil {
				panic(err)
			}
			c.etcdClient = etcdClient
		} else {
			c.etcdClient = etcd.NewClient([]string{c.Config.EtcdAddr})
		}

		go c.writeServiceHeartbeat()
	}

	// write event logs
	go c.writeLogs()

	// attach to exit signals
	c.attachSignals()

	// write the status to standard output
	pid := os.Getpid()
	c.events <- LogEvent{"info", fmt.Sprintf(Header, c.Config.ServiceName, Version, pid), nil, nil}

	// execute the specified work handled by this process
	c.executeWorkFn()
}

// writeServiceHeartbeat will simply set the current configuration with a ttl in etcd, which can be used as a sort of heartbeat detection to
// see which services are alive according to the service name and the machine name and process id
func (c *WorkClient) writeServiceHeartbeat() {
	pid := os.Getpid()
	key := fmt.Sprintf("%s/%s/%s.%d", c.Config.EtcdPrefixKey, c.Config.ServiceName, c.Config.Hostname, pid)
	for {
		select {
		case <-c.heartbeatclosed:
			c.heartbeatTicker.Stop()
			return
		case <-c.heartbeatTicker.C:
			_, err := c.etcdClient.Set(key, c.MarshalledConfig, uint64(c.Config.EtcdHeartbeatTtl))
			if err != nil {
				c.events <- LogEvent{"error", "etcd error", err, nil}
			}
		}
	}
}

func (c *WorkClient) NewGauge(metric string) metrics.Gauge {
	g := metrics.NewGauge()
	metrics.Register(fmt.Sprintf("%s.%s", c.Config.ServiceName, metric), g)
	c.gaugeMap[metric] = g
	return g
}

func (c *WorkClient) NewTimer(metric string) metrics.Timer {
	t := metrics.NewTimer()
	metrics.Register(fmt.Sprintf("%s.%s", c.Config.ServiceName, metric), t)
	c.timeMap[metric] = t
	return t
}

// writeRuntimeStats will simply use influxdb and statsd to write all relevant runtime statistics used for debugging and tracing load for the worker
func (c *WorkClient) writeRuntimeStats() {
	// setup gauges to monitor with metrics
	gauges := []string{"cpu_num", "num_gc", "goroutine_num", "cgo_call_num",
		"gomaxprocs", "memory_alloc", "memory_total_alloc", "memory_mallocs", "memory_frees", "memory_stack", "heap_alloc",
		"heap_sys", "heap_idle", "heap_inuse", "heap_released", "heap_objects"}
	for _, k := range gauges {
		c.NewGauge(k)
	}

	// setup timers to monitor with metrics
	timings := []string{"gc_per_second", "gc_pause_per_second"}
	for _, k := range timings {
		c.NewTimer(k)
	}

	// bind periodic updates to stderr based on configuration
	if c.Config.StdErrMetrics {
		go metrics.Log(metrics.DefaultRegistry, 30e9, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
	}
	// bind periodic updates to graphite
	if c.Config.GraphiteAddr != "" {
		addr, _ := net.ResolveTCPAddr("tcp", c.Config.GraphiteAddr)
		go metrics.Graphite(metrics.DefaultRegistry, 10e9, c.Config.GraphitePrefix, addr)
	}
	// bind to periodic updates to influxdb
	if c.Config.InfluxDbAddr != "" && c.Config.InfluxDbServiceMetricsDb != "" {
		go influxdb.Influxdb(metrics.DefaultRegistry, 10e9, &influxdb.Config{
			Host:     c.Config.InfluxDbAddr,
			Database: c.Config.InfluxDbServiceMetricsDb,
			Username: c.Config.InfluxDbUsername,
			Password: c.Config.InfluxDbPassword,
		})
	}

	for {
		select {
		case <-c.runtimeclosed:
			c.runtimeTicker.Stop()
			return
		case <-c.runtimeTicker.C:
			_, m, _ := GetRuntimeStats()
			if m != nil {
				for k, g := range c.gaugeMap {
					if value, ok := m[k]; ok {
						num, _ := value.(json.Number).Int64()
						g.Update(num)
						if c.stats != nil {
							c.stats.Gauge(k, num)
						}
					}
				}
				for k, t := range c.timeMap {
					if value, ok := m[k]; ok {
						num, _ := value.(json.Number).Float64()
						t.Update(time.Duration(num))
						if c.stats != nil {
							c.stats.Timing(k, int64(num))
						}
					}
				}
			}
		}
	}
}

func (c *WorkClient) writeLogs() {
	// wait for all events to fire so we can log them
	pid := os.Getpid()
	writer := os.Stderr
	redirectNull := false
	colors := true
	if c.Config.StdErrLogFile != "" && c.Config.StdErrLogFile != "/dev/null" {
		f, err := os.OpenFile(c.Config.StdErrLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		} else {
			writer = f
			defer f.Close()
		}
		colors = false
	} else if c.Config.StdErrLogFile == "/dev/null" {
		redirectNull = true
	}
	logger := log.New(writer, "", log.Ldate|log.Ltime)
	for {
		event, ok := <-c.events
		if ok {
			// no need to actually write anything, since the user doesn't care
			if redirectNull {
				continue
			}

			delim := "#"
			if event.Level == "error" {
				delim = "ERROR:"
			}
			prefix := ""
			if colors {
				prefix = "\033[36m"
				if event.Level == "error" {
					prefix = "\033[31m"
				}
			}
			if colors {
				logger.SetPrefix(fmt.Sprintf("%s[%d]\033[m ", prefix, pid))
			} else {
				logger.SetPrefix(fmt.Sprintf("[%d] ", pid))
			}
			msg := fmt.Sprintf("%s %s", delim, event.Message)
			if event.Err != nil {
				msg += fmt.Sprintf(" %v", event.Err)
			}

			logger.Println(msg)
		} else if len(c.events) == 0 {
			break
		}
	}

	close(c.logclosed)
}

// Close the running client appropriately
func (c *WorkClient) Close() {
	c.closeExecuteWorkFn()

	// close the log event buffer, runtime stats, and heartbeat
	close(c.runtimeclosed)
	close(c.heartbeatclosed)
	close(c.events)
	<-c.logclosed

	// close the application
	close(c.quit)
}

// attachSignals will create a channel to OS.Signal to listen for any signup events..etc
func (c *WorkClient) attachSignals() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGKILL,
		os.Interrupt)

	go func() {
		<-sc
		c.Close()
	}()
}
