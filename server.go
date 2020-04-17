package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/lb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	yaml "gopkg.in/yaml.v2"
)

const version = "0.3.0-dev"

var (
	listenAddress = flag.String("web.listen-address", ":9268", "Address to listen on for Prometheus requests.")
	configFile    = flag.String("config.file", "config.yml", "Path to the CrateDB endpoints configuration file.")
	print_version = flag.Bool("version", false, "Print version information.")

	writeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "crate_adapter_write_latency_seconds",
		Help: "How long it took us to respond to write requests.",
	})
	writeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "crate_adapter_write_failed_total",
		Help: "How many write request we returned errors for.",
	})
	writeSamples = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "crate_adapter_write_timeseries_samples",
		Help: "How many samples each written timeseries has.",
	})
	writeCrateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "crate_adapter_write_crate_latency_seconds",
		Help: "Latency for inserts to Crate.",
	})
	writeCrateErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "crate_adapter_write_crate_failed_total",
		Help: "How many inserts to Crate failed.",
	})
	readDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "crate_adapter_read_latency_seconds",
		Help: "How long it took us to respond to read requests.",
	})
	readErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "crate_adapter_read_failed_total",
		Help: "How many read requests we returned errors for.",
	})
	readCrateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "crate_adapter_read_crate_latency_seconds",
		Help: "Latency for selects from Crate.",
	})
	readCrateErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "crate_adapter_read_crate_failed_total",
		Help: "How many selects from Crate failed.",
	})
	readSamples = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "crate_adapter_read_timeseries_samples",
		Help: "How many samples each returned timeseries has.",
	})
)

func init() {
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
}

// Escaping for strings for Crate.io SQL.
var escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "'", "\\'")

// Escape a labelname for use in SQL as a column name.
func escapeLabelName(s string) string {
	return "labels['" + escaper.Replace(s) + "']"
}

// Escape a labelvalue for use in SQL as a string value.
func escapeLabelValue(s string) string {
	return "'" + escaper.Replace(s) + "'"
}

// Convert a read query into a Crate SQL query.
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
			// Crate regexes are not RE2, so there may be small semantic differences here.
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
				ts.Labels = append(ts.Labels, &prompb.Label{Name: string(k), Value: string(metric[model.LabelName(k)])})
			}
			timeseries[metric.String()] = ts
		}
		ts.Samples = append(ts.Samples, &prompb.Sample{Value: v, Timestamp: t})
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

type crateAdapter struct {
	ep endpoint.Endpoint
}

func (ca *crateAdapter) runQuery(q *prompb.Query) ([]*prompb.TimeSeries, error) {
	query, err := queryToSQL(q)
	if err != nil {
		return nil, err
	}

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

func (ca *crateAdapter) handleRead(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(readDuration)
	defer timer.ObserveDuration()

	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.With("err", err).Error("Failed to read body.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		log.With("err", err).Error("Failed to decompress body.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.ReadRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		log.With("err", err).Error("Failed to unmarshal body.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Queries) != 1 {
		log.Error("More than one query sent.")
		http.Error(w, "Can only handle one query.", http.StatusBadRequest)
		return
	}

	result, err := ca.runQuery(req.Queries[0])
	if err != nil {
		log.With("err", err).Error("Failed to run select against Crate.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{Timeseries: result},
		},
	}
	data, err := proto.Marshal(&resp)
	if err != nil {
		log.With("err", err).Error("Failed to marshal response.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(snappy.Encode(nil, data)); err != nil {
		log.With("err", err).Error("Failed to compress response.")
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

func (ca *crateAdapter) handleWrite(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(writeDuration)
	defer timer.ObserveDuration()

	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.With("err", err).Error("Failed to read body")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		log.With("err", err).Error("Failed to decompress body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.WriteRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		log.With("err", err).Error("Failed to unmarshal body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request := writesToCrateRequest(&req)

	writeTimer := prometheus.NewTimer(writeCrateDuration)
	_, err = ca.ep(context.Background(), request)
	writeTimer.ObserveDuration()
	if err != nil {
		writeCrateErrors.Inc()
		log.With("err", err).Error("Failed to write data to Crate.")
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
	EnableTLS        bool   `yaml:"enable_tls"`
	AllowInsecureTLS bool   `yaml:"allow_insecure_tls"`
	MaxConnections   int    `yaml:"max_connections"`
}

type config struct {
	Endpoints []endpointConfig `yaml:"crate_endpoints"`
}

func loadConfig(filename string) (*config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration file: %v", err)
	}
	conf := &config{}
	if err = yaml.UnmarshalStrict(content, conf); err != nil {
		return nil, fmt.Errorf("error unmarshaling YAML: %v", err)
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
		if conf.Endpoints[i].MaxConnections == 0 {
			conf.Endpoints[i].MaxConnections = 5
		}
	}
	return conf, nil
}

func main() {
	flag.Parse()

	if *print_version == true {
		fmt.Println(version)
		return
	}

	conf, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Error loading configuration file %q: %v", *configFile, err)
	}

	subscriber := sd.FixedEndpointer{}
	for _, epConf := range conf.Endpoints {
		subscriber = append(subscriber, newCrateEndpoint(&epConf).endpoint())
	}
	balancer := lb.NewRoundRobin(subscriber)
	// Try each endpoint once.
	retry := lb.Retry(len(conf.Endpoints), 1*time.Minute, balancer)

	ca := crateAdapter{
		ep: retry,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
    <head><title>Crate.io Prometheus Adapter</title></head>
    <body>
    <h1>Crate.io Prometheus Adapter</h1>
    </body>
    </html>`))
	})

	http.HandleFunc("/write", ca.handleWrite)
	http.HandleFunc("/read", ca.handleRead)
	http.Handle("/metrics", promhttp.Handler())
	log.With("address", *listenAddress).Info("Listening")
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
