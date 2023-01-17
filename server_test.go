package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"math"
	"path/filepath"
	"reflect"
	"regexp/syntax"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
)

func TestQueryToSQL(t *testing.T) {
	cases := []struct {
		query *prompb.Query
		sql   string
		err   error
	}{
		{
			query: &prompb.Query{
				Matchers:         []*prompb.LabelMatcher{},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT labels, labels_hash, timestamp, value, "valueRaw" FROM metrics WHERE (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &prompb.Query{
				Matchers: []*prompb.LabelMatcher{
					{Type: prompb.LabelMatcher_EQ, Name: "n", Value: "v"},
					{Type: prompb.LabelMatcher_NEQ, Name: "n", Value: "v"},
					{Type: prompb.LabelMatcher_RE, Name: "n", Value: "v"},
					{Type: prompb.LabelMatcher_NRE, Name: "n", Value: "v"},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT labels, labels_hash, timestamp, value, "valueRaw" FROM metrics WHERE (labels['n'] = 'v') AND (labels['n'] != 'v') AND (labels['n'] ~ '(v)') AND (labels['n'] !~ '(v)' OR labels['n'] IS NULL) AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &prompb.Query{
				Matchers: []*prompb.LabelMatcher{
					// These are not valid label names, but test the escaping anyway.
					{Type: prompb.LabelMatcher_EQ, Name: "n'", Value: "v'"},
					{Type: prompb.LabelMatcher_NEQ, Name: "n'", Value: "v'"},
					{Type: prompb.LabelMatcher_RE, Name: "n'", Value: "v'"},
					{Type: prompb.LabelMatcher_NRE, Name: "n'", Value: "v'"},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT labels, labels_hash, timestamp, value, "valueRaw" FROM metrics WHERE (labels['n\''] = 'v\'') AND (labels['n\''] != 'v\'') AND (labels['n\''] ~ '(v\')') AND (labels['n\''] !~ '(v\')' OR labels['n\''] IS NULL) AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &prompb.Query{
				Matchers: []*prompb.LabelMatcher{
					{Type: prompb.LabelMatcher_EQ, Name: "n", Value: ""},
					{Type: prompb.LabelMatcher_NEQ, Name: "n", Value: ""},
					{Type: prompb.LabelMatcher_RE, Name: "n", Value: ""},
					{Type: prompb.LabelMatcher_NRE, Name: "n", Value: ""},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT labels, labels_hash, timestamp, value, "valueRaw" FROM metrics WHERE (labels['n'] IS NULL) AND (labels['n'] IS NOT NULL) AND (labels['n'] ~ '()' OR labels['n'] IS NULL) AND (labels['n'] !~ '()') AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &prompb.Query{
				Matchers: []*prompb.LabelMatcher{
					{Type: prompb.LabelMatcher_RE, Name: "n", Value: "*"},
				},
			},
			err: &syntax.Error{Code: "missing argument to repetition operator", Expr: "*"},
		},
	}

	for _, c := range cases {
		result, err := queryToSQL(c.query)
		require.Equal(t, c.err, err)
		require.Equal(t, c.sql, result)
	}

}

func TestResponseToTimeseries(t *testing.T) {
	cases := []struct {
		data       *crateReadResponse
		timeseries []*prompb.TimeSeries
	}{
		{
			data: &crateReadResponse{
				rows: []*crateRow{
					{timestamp: time.Unix(0, 1000*1e6).UTC(), valueRaw: int64(math.Float64bits(1)), value: 1, labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "XXX"},
					{timestamp: time.Unix(0, 2000*1e6).UTC(), valueRaw: int64(math.Float64bits(2)), value: 2, labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "XXX"},
					// Value is purposely wrong, so we know we're using valueRaw.
					{timestamp: time.Unix(0, 3000*1e6).UTC(), valueRaw: int64(math.Float64bits(3)), value: 0, labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "XXX"},
					// Test a negative, which has the most significant bit set.
					{timestamp: time.Unix(0, 4000*1e6).UTC(), valueRaw: int64(math.Float64bits(-1)), value: -1, labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "XXX"},
				},
			},
			timeseries: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "metric"},
						&prompb.Label{Name: "job", Value: "j"},
					},
					Samples: []*prompb.Sample{
						{Value: 1, Timestamp: 1000},
						{Value: 2, Timestamp: 2000},
						{Value: 3, Timestamp: 3000},
						{Value: -1, Timestamp: 4000},
					},
				},
			},
		},
		{
			data: &crateReadResponse{
				rows: []*crateRow{
					{timestamp: time.Unix(0, 1000*1e6).UTC(), valueRaw: int64(math.Float64bits(1)), value: 1, labels: model.Metric{"__name__": "a", "job": "j"}, labelsHash: "XXX"},
					{timestamp: time.Unix(0, 1000*1e6).UTC(), valueRaw: int64(math.Float64bits(2)), value: 2, labels: model.Metric{"__name__": "b", "job": "j"}, labelsHash: "XXX"},
				},
			},
			timeseries: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "a"},
						&prompb.Label{Name: "job", Value: "j"},
					},
					Samples: []*prompb.Sample{
						{Value: 1, Timestamp: 1000},
					},
				},
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "b"},
						&prompb.Label{Name: "job", Value: "j"},
					},
					Samples: []*prompb.Sample{
						{Value: 2, Timestamp: 1000},
					},
				},
			},
		},
	}

	for _, c := range cases {
		result := responseToTimeseries(c.data)
		require.Equal(t, c.timeseries, result)
	}

}

func TestWritesToCrateRequest(t *testing.T) {
	cases := []struct {
		series  []*prompb.TimeSeries
		request *crateWriteRequest
	}{
		{
			series: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "metric"},
						&prompb.Label{Name: "job", Value: "j"},
					},
					Samples: []*prompb.Sample{
						{Value: 1, Timestamp: 1000},
						{Value: math.Inf(1), Timestamp: 2000},
						{Value: math.Inf(-1), Timestamp: 3000},
					},
				},
			},
			request: &crateWriteRequest{
				rows: []*crateRow{
					{labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "686aa056b20923af", timestamp: time.Unix(0, 1000*1e6).UTC(), value: 1, valueRaw: 4607182418800017408},
					{labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "686aa056b20923af", timestamp: time.Unix(0, 2000*1e6).UTC(), value: math.Inf(1), valueRaw: 9218868437227405312},
					{labels: model.Metric{"__name__": "metric", "job": "j"}, labelsHash: "686aa056b20923af", timestamp: time.Unix(0, 3000*1e6).UTC(), value: math.Inf(-1), valueRaw: -4503599627370496},
				},
			},
		},
		{
			series: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "a"},
						&prompb.Label{Name: "job", Value: "j"},
					},
					Samples: []*prompb.Sample{
						{Value: 1, Timestamp: 1000},
					},
				},
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "__name__", Value: "b"},
						&prompb.Label{Name: "job", Value: "j"},
						&prompb.Label{Name: "foo", Value: "bar"},
					},
					Samples: []*prompb.Sample{
						{Value: 2, Timestamp: 1000},
					},
				},
			},
			request: &crateWriteRequest{
				rows: []*crateRow{
					{labels: model.Metric{"__name__": "a", "job": "j"}, labelsHash: "ac2e1accf88b4a18", timestamp: time.Unix(0, 1000*1e6).UTC(), value: 1, valueRaw: 4607182418800017408},
					{labels: model.Metric{"__name__": "b", "job": "j", "foo": "bar"}, labelsHash: "d9b64ab8cc80c298", timestamp: time.Unix(0, 1000*1e6).UTC(), value: 2, valueRaw: 4611686018427387904},
				},
			},
		},
		{
			series: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						&prompb.Label{Name: "\"'", Value: "\"'"},
					},
					Samples: []*prompb.Sample{
						{Value: 1, Timestamp: 1000},
					},
				},
			},
			request: &crateWriteRequest{
				rows: []*crateRow{
					{labels: model.Metric{"\"'": "\"'"}, labelsHash: "fd0b18b0901a3291", timestamp: time.Unix(0, 1000*1e6).UTC(), value: 1, valueRaw: 4607182418800017408},
				},
			},
		},
	}

	for _, c := range cases {
		result := writesToCrateRequest(&prompb.WriteRequest{Timeseries: c.series})
		require.Equal(t, c.request, result)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		file        string
		shouldFail  bool
		errContains string
		config      *config
	}{
		{
			file:       filepath.Join("fixtures", "config_good.yml"),
			shouldFail: false,
			config: &config{
				Endpoints: []endpointConfig{
					{
						Host:             "host1",
						Port:             1,
						User:             "user1",
						Password:         "password1",
						Schema:           "schema1",
						ConnectTimeout:   30,
						MaxConnections:   10,
						EnableTLS:        false,
						AllowInsecureTLS: false,
					},
					{
						Host:             "host2",
						Port:             2,
						User:             "user2",
						Password:         "password2",
						Schema:           "schema2",
						ConnectTimeout:   30,
						MaxConnections:   20,
						EnableTLS:        true,
						AllowInsecureTLS: false,
					},
					{
						Host:             "localhost",
						Port:             5432,
						User:             "crate",
						Password:         "",
						Schema:           "",
						ConnectTimeout:   10,
						MaxConnections:   5,
						EnableTLS:        true,
						AllowInsecureTLS: true,
					},
				},
			},
		},
		{
			file:        filepath.Join("fixtures", "config_no_endpoints.yml"),
			shouldFail:  true,
			errContains: "no CrateDB endpoints provided",
		},
		{
			file:        filepath.Join("fixtures", "config_unknown_fields.yml"),
			shouldFail:  true,
			errContains: "field unknown_fields not found",
		},
		{
			file:        filepath.Join("fixtures", "config_invalid_yaml.yml"),
			shouldFail:  true,
			errContains: "error unmarshaling YAML",
		},
		{
			file:       filepath.Join("fixtures", "config_missing_file.yml"),
			shouldFail: false,
			config: &config{
				Endpoints: []endpointConfig{
					{
						Host:             "localhost",
						Port:             5432,
						User:             "crate",
						Password:         "",
						Schema:           "",
						ConnectTimeout:   10,
						MaxConnections:   5,
						EnableTLS:        false,
						AllowInsecureTLS: false,
					},
				},
			},
		},
	}

	for _, test := range tests {
		conf, err := loadConfig(test.file)
		// Check correct loading.
		if err != nil {
			if test.shouldFail {
				if !strings.Contains(err.Error(), test.errContains) {
					t.Errorf("%q: expected error %q to contain %q", test.file, err, test.errContains)
				}
				continue
			}
			t.Errorf("%q: unexpected error: %v", test.file, err)
		} else {
			if test.shouldFail {
				t.Errorf("%q: expected error, got none", test.file)
			}
		}

		// Check contents.
		if !reflect.DeepEqual(test.config, conf) {
			t.Errorf("%q: unexpected config contents;\n\nwant:\n\n%v\n\ngot:\n\n%v", test.file, test.config, conf)
		}
	}
}

func TestExportedMetrics(t *testing.T) {

	metrics, _ := prometheus.DefaultGatherer.Gather()
	for _, metric := range metrics {
		name := metric.GetName()
		if !strings.HasPrefix(name, "cratedb_prometheus_adapter_") &&
			!strings.HasPrefix(name, "go_") &&
			!strings.HasPrefix(name, "process_") {
			message := fmt.Sprintf("Exported metrics prefix does not match expectation for '%s'", name)
			t.Fatal(message)
		}
	}

}

func TestBuiltinConfig(t *testing.T) {

	referenceConfig := &config{
		Endpoints: []endpointConfig{
			{
				Host:             "localhost",
				Port:             5432,
				User:             "crate",
				Password:         "",
				Schema:           "",
				ConnectTimeout:   10,
				MaxConnections:   5,
				EnableTLS:        false,
				AllowInsecureTLS: false,
			},
		},
	}

	builtinConfig, _ := loadConfig("")
	if !reflect.DeepEqual(referenceConfig, builtinConfig) {
		t.Errorf("unexpected config contents;\n\nwant:\n\n%v\n\ngot:\n\n%v", referenceConfig, builtinConfig)
	}
}
