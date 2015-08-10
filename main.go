package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nyxtom/workclient"
)

func attachWebFlags() func() *WebConfig {
	// standard configurations (statsd, http endpoint, reconnection timeout)
	var statsdAddr = flag.String("statsd_addr", "", "address to statsd for publishing statistics about the stream")
	var statsdInterval = flag.Int("statsd_interval", 2, "flush interval for the statsd client to the endpoint in seconds")
	var statsdPrefix = flag.String("statsd_prefix", "", "statsd prefix for the webterm")

	var graphiteAddr = flag.String("graphite_addr", "", "graphite address for the webterm metrics")
	var graphitePrefix = flag.String("graphite_prefix", "", "graphite prefix for the webterm")

	var stdErrLog = flag.String("stderr_logfile", "", "writes all stderr log output to the given file or endpoint")

	var serviceName = flag.String("service_name", "webterm", "name of the service that this configuration is running")
	var hostname = flag.String("hostname", "", "resolved hostname of the service should the OS level hostname be unavailable")

	// etcd configuration endpoint for service registry
	var etcdAddr = flag.String("etcd_addr", "", "host address for etcd for service registration")
	var etcdCaCert = flag.String("etcd_cacert", "", "cacert for tls client associated with etcd connections")
	var etcdTlsKey = flag.String("etcd_tlskey", "", "tlskey associated with clients connected to etcd")
	var etcdTlsCert = flag.String("etcd_tlscert", "", "tlscert associated with clients connected to etcd")
	var etcdPrefixKey = flag.String("etcd_prefix_key", "", "etcd prefix key associated with the registered services")
	var etcdHeartbeatTtl = flag.Int("etcd_heartbeat_ttl", 3, "time in seconds between heartbeat service checks")

	// web configuration values
	var webAddr = flag.String("web_addr", ":5000", "primary web address location to listen on")
	var readTimeout = flag.Duration("web_read_timeout", 10*time.Second, "read connection timeout for the web host")
	var writeTimeout = flag.Duration("web_write_timeout", 10*time.Second, "write connection timeout for the web host")
	var maxHeaderBytes = flag.Int("web_max_header_bytes", 1<<16, "maximum header bytes for the web host")

	// broadcast client configuration
	var bPort = flag.Int("broadcast_port", 7337, "primary broadcast server location port")
	var bIP = flag.String("broadcast_ip", "127.0.0.1", "primary broadcast server location host")
	var bProtocol = flag.String("broadcast_proto", "redis", "primary broadcast server protocol")

	// configuration file option
	var configFile = flag.String("config", "", "configuration file to load as an alternative to explicit flags (toml formatted)")
	return func() *WebConfig {
		cfg := &WebConfig{workclient.Config{*statsdAddr, *statsdInterval, *statsdPrefix,
			*stdErrLog, *graphiteAddr, *graphitePrefix,
			*etcdAddr, *etcdCaCert, *etcdTlsKey, *etcdTlsCert, *etcdPrefixKey, *etcdHeartbeatTtl,
			*serviceName, *hostname, *webAddr, *readTimeout, *writeTimeout, *maxHeaderBytes}, *bPort, *bIP, *bProtocol}

		// load configuration file data from toml format appropriately
		return loadConfig(cfg, *configFile)
	}
}

func loadConfig(cfg *WebConfig, configFile string) *WebConfig {
	if configFile != "" {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			panic(err)
		}

		_, err = toml.Decode(string(data), cfg)
		if err != nil {
			panic(err)
		}
	}

	return cfg
}

func main() {
	var fd = flag.Int("fd", 0, "existing listening socket file descriptor")
	var background = flag.Bool("background", false, "run the process in the background")
	appConfigFn := attachWebFlags()
	flag.Parse()

	if *background {
		args := []string{}
		for _, k := range os.Args {
			if k != "--background" && k != "-background" {
				args = append(args, k)
			}
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			log.Fatalf(err.Error())
		}
	} else {
		ServeWeb(appConfigFn(), *fd, os.Args)
	}
}
