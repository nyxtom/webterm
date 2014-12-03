package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/BurntSushi/toml"
	"github.com/nyxtom/kingpin"
	"github.com/nyxtom/webterm"
)

func attachWebFlags(cmd *kingpin.CmdClause) func() *webterm.AppConfiguration {
	// standard configurations (statsd, http endpoint, reconnection timeout)
	var statsdAddr = cmd.Flag("statsd_addr", "address to statsd for publishing statistics about the stream").String()
	var statsdInterval = cmd.Flag("statsd_interval", "flush interval for the statsd client to the endpoint in seconds").Default("2").Int()
	var statsdPrefix = cmd.Flag("statsd_prefix", "statsd prefix for the webterm").Default("webterm.").String()
	var graphiteAddr = cmd.Flag("graphite_addr", "graphite address for the webterm metrics").Default("").String()
	var graphitePrefix = cmd.Flag("graphite_prefix", "graphite prefix for the webterm").Default("webterm-metrics.").String()
	var influxDbAddr = cmd.Flag("influxdb_addr", "influxdb address for the webterm metrics").String()
	var influxDbDatabase = cmd.Flag("influxdb_database", "influxdb database for the webterm metrics").String()
	var influxDbUsername = cmd.Flag("influxdb_username", "influxdb username for the webterm metrics").String()
	var influxDbPassword = cmd.Flag("influxdb_password", "influxdb password for the webterm metrics").String()
	var influxDbServiceMetricsDb = cmd.Flag("influxdb_service_metrics_db", "influxdb service metrics database name").String()
	var stdErrLog = cmd.Flag("stderr_logfile", "writes all stderr log output to the given file or endpoint").String()
	var stdErrMetrics = cmd.Flag("stderr_metrics", "periodically writes all service metrics to human readable form on stderr").Default("false").Bool()
	var serviceName = cmd.Flag("service_name", "name of the service that this configuration is running").Default("webterm").String()
	var hostname = cmd.Flag("hostname", "resolved hostname of the service should the OS level hostname be unavailable").String()

	// etcd configuration endpoint for service registry
	var etcdAddr = cmd.Flag("etcd_addr", "host address for etcd for service registration").String()
	var etcdCaCert = cmd.Flag("etcd_cacert", "cacert for tls client associated with etcd connections").String()
	var etcdTlsKey = cmd.Flag("etcd_tlskey", "tlskey associated with clients connected to etcd").String()
	var etcdTlsCert = cmd.Flag("etcd_tlscert", "tlscert associated with clients connected to etcd").String()
	var etcdPrefixKey = cmd.Flag("etcd_prefix_key", "etcd prefix key associated with the registered services").Default("/services/webterm").String()
	var etcdHeartbeatTtl = cmd.Flag("etcd_heartbeat_ttl", "time in seconds between heartbeat service checks").Default("3").Int()

	// web configuration values
	var webAddr = cmd.Flag("web_addr", "primary web address location to listen on").Default(":5000").String()
	var readTimeout = cmd.Flag("web_read_timeout", "read connection timeout for the web host").Default("10s").Duration()
	var writeTimeout = cmd.Flag("web_write_timeout", "write connection timeout for the web host").Default("10s").Duration()
	var maxHeaderBytes = cmd.Flag("web_max_header_bytes", "maximum header bytes for the web host").Default("65536").Int()

	// configuration file option
	var configFile = cmd.Flag("config", "configuration file to load as an alternative to explicit flags (toml formatted)").Default("").String()
	return func() *webterm.AppConfiguration {
		cfg := &webterm.AppConfiguration{false, "", "", *statsdAddr, *statsdInterval, *statsdPrefix,
			*stdErrLog, *stdErrMetrics, *graphiteAddr, *graphitePrefix,
			*influxDbAddr, *influxDbDatabase, *influxDbUsername, *influxDbPassword, *influxDbServiceMetricsDb,
			*etcdAddr, *etcdCaCert, *etcdTlsKey, *etcdTlsCert, *etcdPrefixKey, *etcdHeartbeatTtl,
			*serviceName, *hostname, *webAddr, *readTimeout, *writeTimeout, *maxHeaderBytes}

		// load configuration file data from toml format appropriately
		return loadConfig(cfg, *configFile)
	}
}

func loadConfig(cfg *webterm.AppConfiguration, configFile string) *webterm.AppConfiguration {
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
	var app = kingpin.New("webterm", "")
	app.Version("0.1.0")
	app.SetCompactUsage(true)
	app.SetHelpCmd(false)
	app.SetHelpUsageOnError(true)

	// web command
	var webCmd = app.Command("web", "web host for the web terminal front-end application")
	appConfigFn := attachWebFlags(webCmd)
	var graceful = webCmd.Flag("graceful", "").Default("false").Bool()
	var background = webCmd.Flag("background", "run the process in the background").Default("false").Bool()

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case "web":
		{
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
				cfg := appConfigFn()
				client := webterm.NewWebServer(cfg, *graceful, os.Args)
				marshalledConfig, _ := json.Marshal(&cfg)
				client.MarshalledConfig = string(marshalledConfig)
				client.Run()
			}
		}
	}
}
