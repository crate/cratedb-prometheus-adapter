package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/prometheus/common/promslog"
	"io/ioutil"
	slog "log/slog"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/lb"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	yaml "gopkg.in/yaml.v2"
)

const version = "0.5.3"

var (
	listenAddress       = flag.String("web.listen-address", ":9268", "Address to listen on for Prometheus requests.")
	configFile          = flag.String("config.file", "", "Path to the CrateDB endpoints configuration file.")
	metricsExportPrefix = flag.String("metrics.export.prefix", "cratedb_prometheus_adapter_", "Prefix for exported CrateDB metrics.")
	makeConfig          = flag.Bool("config.make", false, "Print configuration file blueprint to stdout.")
	printVersion        = flag.Bool("version", false, "Print version information.")

	writeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%swrite_latency_seconds", *metricsExportPrefix),
		Help: "How long it took to respond to write requests.",
	})
	writeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%swrite_failed_total", *metricsExportPrefix),
		Help: "How many write request returned errors.",
	})
	writeSamples = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: fmt.Sprintf("%swrite_timeseries_samples", *metricsExportPrefix),
		Help: "How many samples each written timeseries has.",
	})
	writeCrateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%swrite_crate_latency_seconds", *metricsExportPrefix),
		Help: "Latency for inserts to CrateDB.",
	})
	writeCrateErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%swrite_crate_failed_total", *metricsExportPrefix),
		Help: "How many inserts to CrateDB failed.",
	})
	readDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%sread_latency_seconds", *metricsExportPrefix),
		Help: "How long it took to respond to read requests.",
	})
	readErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%sread_failed_total", *metricsExportPrefix),
		Help: "How many read requests returned errors.",
	})
	readCrateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%sread_crate_latency_seconds", *metricsExportPrefix),
		Help: "Latency for selects from CrateDB.",
	})
	readCrateErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%sread_crate_failed_total", *metricsExportPrefix),
		Help: "How many selects from CrateDB failed.",
	})
	readSamples = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: fmt.Sprintf("%sread_timeseries_samples", *metricsExportPrefix),
		Help: "How many samples each returned timeseries has.",
	})
)

// Module-wide `logger` variable, initialized by `setupLogging()`.
var logger *slog.Logger

func init() {
	setupLogging()
	prometheus.MustRegister(writeDuration)
	prometheus.MustRegister(writeErrors)
	prometheus.MustRegister(writeSamples)
	prometheus.MustRegister(writeCrateDuration)
	prometheus.MustRegister(writeCrateErrors)
	prometheus.MustRegister(readDuration)
	prometheus.MustRegister(readErrors)
	prometheus.MustRegister(readSamples)
	prometheus.MustRegister(readCrateDuration)
	prometheus.MustRegister(readCrateErrors)
	logger.Info("Initialized CrateDB Prometheus Adapter", "version", version)
}

// Escaping for strings for Crate.io SQL.
var escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "'", "\\'")

// Set up promslog logger.
func setupLogging() {
	logLevel := promslog.AllowedLevel{}
	logLevel.Set("debug")
	logConfig := &promslog.Config{Level: &logLevel}
	logger = promslog.New(logConfig)
}

// Escape a labelname for use in SQL as a column name.
func escapeLabelName(s string) string {
	return "labels['" + escaper.Replace(s) + "']"
}

// Escape a labelvalue for use in SQL as a string value.
func escapeLabelValue(s string) string {
	return "'" + escaper.Replace(s) + "'"
}

// Convert a read query into a CrateDB SQL query.
func queryToSQL(q *prompb.Query) (string, error) {
	selectors := make([]string, 0, len(q.Matchers)+2)
	for _, m := range q.Matchers {
		switch m.Type {
		case prompb.LabelMatcher_EQ:
			if m.Value == "" {
				// Empty labels are recorded as NULL.
				// In PromQL, empty labels and missing labels are the same thing.
				selectors = append(selectors, fmt.Sprintf("(%s IS NULL)", escapeLabelName(m.Name)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s = %s)", escapeLabelName(m.Name), escapeLabelValue(m.Value)))
			}
		case prompb.LabelMatcher_NEQ:
			if m.Value == "" {
				selectors = append(selectors, fmt.Sprintf("(%s IS NOT NULL)", escapeLabelName(m.Name)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s != %s)", escapeLabelName(m.Name), escapeLabelValue(m.Value)))
			}
		case prompb.LabelMatcher_RE:
			re := "(" + m.Value + ")"
			matchesEmpty, err := regexp.MatchString(re, "")
			if err != nil {
				return "", err
			}
			// CrateDB regexes are not RE2, so there may be small semantic differences here.
			if matchesEmpty {
				selectors = append(selectors, fmt.Sprintf("(%s ~ %s OR %s IS NULL)", escapeLabelName(m.Name), escapeLabelValue(re), escapeLabelName(m.Name)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s ~ %s)", escapeLabelName(m.Name), escapeLabelValue(re)))
			}
		case prompb.LabelMatcher_NRE:
			re := "(" + m.Value + ")"
			matchesEmpty, err := regexp.MatchString(re, "")
			if err != nil {
				return "", err
			}
			if matchesEmpty {
				selectors = append(selectors, fmt.Sprintf("(%s !~ %s)", escapeLabelName(m.Name), escapeLabelValue(re)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s !~ %s OR %s IS NULL)", escapeLabelName(m.Name), escapeLabelValue(re), escapeLabelName(m.Name)))
			}
		}
	}
	selectors = append(selectors, fmt.Sprintf("(timestamp <= %d)", q.EndTimestampMs))
	selectors = append(selectors, fmt.Sprintf("(timestamp >= %d)", q.StartTimestampMs))

	return fmt.Sprintf(`SELECT labels, labels_hash, timestamp, value, "valueRaw" FROM metrics WHERE %s ORDER BY timestamp`, strings.Join(selectors, " AND ")), nil
}

func responseToTimeseries(data *crateReadResponse) []*prompb.TimeSeries {
	timeseries := map[string]*prompb.TimeSeries{}
	for _, row := range data.rows {
		metric := model.Metric{}
		for k, v := range row.labels {
			metric[model.LabelName(k)] = model.LabelValue(v)
		}

		t := row.timestamp.UnixNano() / 1e6
		v := math.Float64frombits(uint64(row.valueRaw))

		ts, ok := timeseries[metric.String()]
		if !ok {
			ts = &prompb.TimeSeries{}
			labelnames := make([]string, 0, len(metric))
			for k := range metric {
				labelnames = append(labelnames, string(k))
			}
			sort.Strings(labelnames) // Sort for unittests.
			for _, k := range labelnames {
				ts.Labels = append(ts.Labels, prompb.Label{Name: string(k), Value: string(metric[model.LabelName(k)])})
			}
			timeseries[metric.String()] = ts
		}
		ts.Samples = append(ts.Samples, prompb.Sample{Value: v, Timestamp: t})
	}

	names := make([]string, 0, len(timeseries))
	for k := range timeseries {
		names = append(names, k)
	}
	sort.Strings(names)
	resp := make([]*prompb.TimeSeries, 0, len(timeseries))
	for _, name := range names {
		readSamples.Observe(float64(len(timeseries[name].Samples)))
		resp = append(resp, timeseries[name])
	}
	return resp
}

type crateDbPrometheusAdapter struct {
	ep endpoint.Endpoint
}

func (ca *crateDbPrometheusAdapter) runQuery(q *prompb.Query) ([]*prompb.TimeSeries, error) {
	query, err := queryToSQL(q)
	if err != nil {
		return nil, err
	}

	logger.Debug("runQuery", "stmt", query)
	request := &crateReadRequest{stmt: query}

	timer := prometheus.NewTimer(readCrateDuration)
	result, err := ca.ep(context.Background(), request)
	timer.ObserveDuration()
	if err != nil {
		readCrateErrors.Inc()
		return nil, err
	}
	return responseToTimeseries(result.(*crateReadResponse)), nil
}

func (ca *crateDbPrometheusAdapter) handleRead(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(readDuration)
	defer timer.ObserveDuration()

	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		logger.Error("Failed to decompress body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.ReadRequest
	if err := req.Unmarshal(reqBuf); err != nil {
		logger.Error("Failed to unmarshal body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Queries) != 1 {
		logger.Error("More than one query sent")
		http.Error(w, "Can only handle one query.", http.StatusBadRequest)
		return
	}

	result, err := ca.runQuery(req.Queries[0])
	if err != nil {
		logger.Warn("Failed to run select against CrateDB", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{Timeseries: result},
		},
	}
	data, err := resp.Marshal()
	if err != nil {
		logger.Error("Failed to marshal response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(snappy.Encode(nil, data)); err != nil {
		logger.Error("Failed to compress response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func writesToCrateRequest(req *prompb.WriteRequest) *crateWriteRequest {
	request := &crateWriteRequest{
		rows: make([]*crateRow, 0, len(req.Timeseries)),
	}

	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			metric[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}
		fp := metric.Fingerprint().String()

		for _, s := range ts.Samples {
			request.rows = append(request.rows, &crateRow{
				labels:     metric,
				labelsHash: fp,
				timestamp:  time.Unix(0, s.Timestamp*1e6).UTC(),
				value:      s.Value,
				// Crate.io can't handle full NaN values as required by Prometheus 2.0,
				// so store the raw bits as an int64.
				valueRaw: int64(math.Float64bits(s.Value)),
			})
		}
		writeSamples.Observe(float64(len(ts.Samples)))
	}
	return request
}

func (ca *crateDbPrometheusAdapter) handleWrite(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(writeDuration)
	defer timer.ObserveDuration()

	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read body", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		logger.Error("Failed to decompress body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.WriteRequest
	if err := req.Unmarshal(reqBuf); err != nil {
		logger.Error("Failed to unmarshal body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request := writesToCrateRequest(&req)

	writeTimer := prometheus.NewTimer(writeCrateDuration)
	_, err = ca.ep(context.Background(), request)
	writeTimer.ObserveDuration()
	if err != nil {
		writeCrateErrors.Inc()
		logger.Error("Failed to write data to CrateDB", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type endpointConfig struct {
	Host             string `yaml:"host"`
	Port             uint16 `yaml:"port"`
	User             string `yaml:"user"`
	Password         string `yaml:"password"`
	Schema           string `yaml:"schema"`
	MaxConnections   int    `yaml:"max_connections"`
	ReadPoolSize     int    `yaml:"read_pool_size_max"`
	WritePoolSize    int    `yaml:"write_pool_size_max"`
	ConnectTimeout   int    `yaml:"connect_timeout"`
	ReadTimeout      int    `yaml:"read_timeout"`
	WriteTimeout     int    `yaml:"write_timeout"`
	EnableTLS        bool   `yaml:"enable_tls"`
	AllowInsecureTLS bool   `yaml:"allow_insecure_tls"`
}

func (ep *endpointConfig) toDSN() string {
	// Convert endpointConfig data to libpq-compatible DSN-style connection string.
	var params []string
	if ep.Host != "" {
		params = append(params, fmt.Sprintf("host=%s", ep.Host))
	}
	if ep.Port != 0 {
		params = append(params, fmt.Sprintf("port=%v", ep.Port))
	}
	if ep.User != "" {
		params = append(params, fmt.Sprintf("user=%s", ep.User))
	}
	if ep.Password != "" {
		params = append(params, fmt.Sprintf("password=%s", ep.Password))
	}
	if ep.Schema != "" {
		params = append(params, fmt.Sprintf("database=%s", ep.Schema))
	}
	if ep.ConnectTimeout != 0 {
		params = append(params, fmt.Sprintf("connect_timeout=%v", ep.ConnectTimeout))
	}
	if ep.MaxConnections != 0 {
		params = append(params, fmt.Sprintf("pool_max_conns=%v", ep.MaxConnections))
	}
	return strings.Join(params, " ")
}

type config struct {
	Endpoints []endpointConfig `yaml:"cratedb_endpoints"`
}

func (c *config) toString() string {
	var ep []string
	for _, e := range c.Endpoints {
		ep = append(ep, fmt.Sprintf("%s@%s:%d/%s", e.User, e.Host, e.Port, e.Schema))
	}
	return strings.Join(ep, ",")
}

func (c *config) toYaml() string {
	data, err := yaml.Marshal(c)
	if err != nil {
		logger.Error("Serialization to YAML failed", "err", err)
	}
	return string(data)
}

func loadConfig(filename string) (*config, error) {
	conf := &config{}
	if filename != "" {
		logger.Error("Reading configuration from file", "filename", filename)
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("reading configuration file %q failed: %v", filename, err)
		} else {
			if err = yaml.UnmarshalStrict(content, conf); err != nil {
				return nil, fmt.Errorf("error unmarshaling YAML: %v", err)
			}
		}
	} else {
		logger.Error("No configuration file used, falling back to built-in configuration")
		item := endpointConfig{}
		conf.Endpoints = []endpointConfig{item}
	}

	if len(conf.Endpoints) == 0 {
		return nil, fmt.Errorf("no CrateDB endpoints provided in configuration file")
	}
	for i := range conf.Endpoints {
		if conf.Endpoints[i].Host == "" {
			conf.Endpoints[i].Host = "localhost"
		}
		if conf.Endpoints[i].Port == 0 {
			conf.Endpoints[i].Port = 5432
		}
		if conf.Endpoints[i].User == "" {
			conf.Endpoints[i].User = "crate"
		}
		if conf.Endpoints[i].ConnectTimeout == 0 {
			conf.Endpoints[i].ConnectTimeout = 10
		}
		if conf.Endpoints[i].ReadTimeout == 0 {
			conf.Endpoints[i].ReadTimeout = 5
		}
		if conf.Endpoints[i].WriteTimeout == 0 {
			conf.Endpoints[i].WriteTimeout = 5
		}
	}
	return conf, nil
}

func builtinConfig() *config {
	blueprint, _ := loadConfig("")
	return blueprint
}

func main() {

	logger.Info("Starting CrateDB Prometheus Adapter", "version", version)

	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		return
	}

	if *makeConfig {
		fmt.Println(builtinConfig().toYaml())
		return
	}

	conf, err := loadConfig(*configFile)
	if err != nil {
		logger.Error("Error loading configuration", "config", *configFile, "err", err)
	}

	subscriber := sd.FixedEndpointer{}
	for _, epConf := range conf.Endpoints {
		subscriber = append(subscriber, newCrateEndpoint(&epConf).endpoint())
	}
	balancer := lb.NewRoundRobin(subscriber)
	// Try each endpoint once.
	retry := lb.Retry(len(conf.Endpoints), 1*time.Minute, balancer)

	ca := crateDbPrometheusAdapter{
		ep: retry,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
    <head><title>CrateDB Prometheus Adapter</title></head>
    <body>
    <h1>CrateDB Prometheus Adapter</h1>
    </body>
    </html>`))
	})

	http.HandleFunc("/write", ca.handleWrite)
	http.HandleFunc("/read", ca.handleRead)
	http.Handle("/metrics", promhttp.Handler())
	logger.Info("Listening ...", "address", *listenAddress)
	logger.Info("Connecting ...", "endpoints", conf.toString())
	listen_error := http.ListenAndServe(*listenAddress, nil)
	logger.Info("Final outcome", "err", listen_error)
}
