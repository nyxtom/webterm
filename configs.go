package webterm

import "time"

type AppConfiguration struct {
	// Standard configuration values
	StdOutput                bool   `toml:"stdout"`
	StdOutDateLayout         string `toml:"stdout_date_layout"`
	StdOutFile               string `toml:"stdout_file"`
	StatsdAddr               string `toml:"statsd_addr"`
	StatsdInterval           int    `toml:"statsd_interval"`
	StatsdPrefix             string `toml:"statsd_prefix"`
	StdErrLogFile            string `toml:"stderr_logfile"`
	StdErrMetrics            bool   `toml:"stderr_metrics"`
	GraphiteAddr             string `toml:"graphite_addr"`
	GraphitePrefix           string `toml:"graphite_prefix"`
	InfluxDbAddr             string `toml:"influxdb_addr"`
	InfluxDbDatabase         string `toml:"influxdb_database"`
	InfluxDbUsername         string `toml:"influxdb_username"`
	InfluxDbPassword         string `toml:"influxdb_password"`
	InfluxDbServiceMetricsDb string `toml:"influxdb_service_metrics_db"`
	EtcdAddr                 string `toml:"etcd_addr"`
	EtcdCaCert               string `toml:"etcd_cacert"`
	EtcdTlsKey               string `toml:"etcd_tlskey"`
	EtcdTlsCert              string `toml:"etcd_tlscert"`
	EtcdPrefixKey            string `toml:"etcd_prefix_key"`
	EtcdHeartbeatTtl         int    `toml:"etcd_heartbeat_ttl"`
	ServiceName              string `toml:"service_name"`
	Hostname                 string `toml:"hostname"`

	// web configuration
	WebAddr        string        `toml:"web_addr" default:"0.0.0.0:5000"`
	ReadTimeout    time.Duration `toml:"web_read_timeout" default:"10s"`
	WriteTimeout   time.Duration `toml:"web_write_timeout" default:"10s"`
	MaxHeaderBytes int           `toml:"web_max_header_bytes" default:"65536"`
}
