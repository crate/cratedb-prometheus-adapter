package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp/syntax"
	"testing"

	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/require"
)

func TestQueryToSQL(t *testing.T) {
	cases := []struct {
		query *remote.Query
		sql   string
		err   error
	}{
		{
			query: &remote.Query{
				Matchers:         []*remote.LabelMatcher{},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT * from metrics WHERE (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &remote.Query{
				Matchers: []*remote.LabelMatcher{
					{Type: remote.MatchType_EQUAL, Name: "n", Value: "v"},
					{Type: remote.MatchType_NOT_EQUAL, Name: "n", Value: "v"},
					{Type: remote.MatchType_REGEX_MATCH, Name: "n", Value: "v"},
					{Type: remote.MatchType_REGEX_NO_MATCH, Name: "n", Value: "v"},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT * from metrics WHERE ("ln" = 'v') AND ("ln" != 'v') AND ("ln" ~ '^(?:v)$') AND ("ln" !~ '^(?:v)$' OR "ln" IS NULL) AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &remote.Query{
				Matchers: []*remote.LabelMatcher{
					// These are not valid label names, but test the escaping anyway.
					{Type: remote.MatchType_EQUAL, Name: "n\"", Value: "v'"},
					{Type: remote.MatchType_NOT_EQUAL, Name: "n\"", Value: "v'"},
					{Type: remote.MatchType_REGEX_MATCH, Name: "n\"", Value: "v'"},
					{Type: remote.MatchType_REGEX_NO_MATCH, Name: "n\"", Value: "v'"},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT * from metrics WHERE ("ln\"" = 'v\'') AND ("ln\"" != 'v\'') AND ("ln\"" ~ '^(?:v\')$') AND ("ln\"" !~ '^(?:v\')$' OR "ln\"" IS NULL) AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &remote.Query{
				Matchers: []*remote.LabelMatcher{
					{Type: remote.MatchType_EQUAL, Name: "n", Value: ""},
					{Type: remote.MatchType_NOT_EQUAL, Name: "n", Value: ""},
					{Type: remote.MatchType_REGEX_MATCH, Name: "n", Value: ""},
					{Type: remote.MatchType_REGEX_NO_MATCH, Name: "n", Value: ""},
				},
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
			sql: `SELECT * from metrics WHERE ("ln" IS NULL) AND ("ln" IS NOT NULL) AND ("ln" ~ '^(?:)$' OR "ln" IS NULL) AND ("ln" !~ '^(?:)$') AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
		{
			query: &remote.Query{
				Matchers: []*remote.LabelMatcher{
					{Type: remote.MatchType_REGEX_MATCH, Name: "n", Value: "*"},
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
	floatToNumber := func(f float64) json.Number {
		return json.Number(fmt.Sprintf("%d", int64(math.Float64bits(f))))
	}
	intToNumber := func(i int64) json.Number {
		return json.Number(fmt.Sprintf("%d", i))
	}
	cases := []struct {
		data       *crateResponse
		timeseries []*remote.TimeSeries
	}{
		{
			data: &crateResponse{
				Cols: []string{"timestamp", "valueRaw", "value", "l__name__", "ljob", "lempty"},
				Rows: [][]interface{}{
					{intToNumber(1000), floatToNumber(1), 1, "metric", "j", nil},
					{intToNumber(2000), floatToNumber(2), 2, "metric", "j", nil},
					// Value is purposely wrong, so we know we're using valueRaw.
					{intToNumber(3000), floatToNumber(3), 0, "metric", "j", nil},
					// Test a negative, which has the most significant bit set.
					{intToNumber(4000), floatToNumber(-1), -1, "metric", "j", nil},
				},
			},
			timeseries: []*remote.TimeSeries{
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "metric"},
						&remote.LabelPair{Name: "job", Value: "j"},
					},
					Samples: []*remote.Sample{
						{Value: 1, TimestampMs: 1000},
						{Value: 2, TimestampMs: 2000},
						{Value: 3, TimestampMs: 3000},
						{Value: -1, TimestampMs: 4000},
					},
				},
			},
		},
		{
			data: &crateResponse{
				Cols: []string{"timestamp", "valueRaw", "value", "l__name__", "ljob", "lempty"},
				Rows: [][]interface{}{
					{intToNumber(1000), floatToNumber(1), 1, "a", "j", nil},
					{intToNumber(1000), floatToNumber(2), 2, "b", "j", nil},
				},
			},
			timeseries: []*remote.TimeSeries{
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "a"},
						&remote.LabelPair{Name: "job", Value: "j"},
					},
					Samples: []*remote.Sample{
						{Value: 1, TimestampMs: 1000},
					},
				},
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "b"},
						&remote.LabelPair{Name: "job", Value: "j"},
					},
					Samples: []*remote.Sample{
						{Value: 2, TimestampMs: 1000},
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
		series  []*remote.TimeSeries
		request *crateRequest
	}{
		{
			series: []*remote.TimeSeries{
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "metric"},
						&remote.LabelPair{Name: "job", Value: "j"},
					},
					Samples: []*remote.Sample{
						{Value: 1, TimestampMs: 1000},
						{Value: 2, TimestampMs: 2000},
						{Value: -1, TimestampMs: 3000},
					},
				},
			},
			request: &crateRequest{
				Stmt: `INSERT INTO metrics ("l__name__", "ljob", "value", "valueRaw", "timestamp") VALUES (?, ?,  ?, ?, ?)`,
				BulkArgs: [][]interface{}{
					{"metric", "j", "1.000000", int64(4607182418800017408), int64(1000)},
					{"metric", "j", "2.000000", int64(4611686018427387904), int64(2000)},
					{"metric", "j", "-1.000000", int64(-4616189618054758400), int64(3000)},
				},
			},
		},
		{
			series: []*remote.TimeSeries{
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "a"},
						&remote.LabelPair{Name: "job", Value: "j"},
					},
					Samples: []*remote.Sample{
						{Value: 1, TimestampMs: 1000},
					},
				},
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "__name__", Value: "b"},
						&remote.LabelPair{Name: "job", Value: "j"},
						&remote.LabelPair{Name: "foo", Value: "bar"},
					},
					Samples: []*remote.Sample{
						{Value: 2, TimestampMs: 1000},
					},
				},
			},
			request: &crateRequest{
				Stmt: `INSERT INTO metrics ("l__name__", "lfoo", "ljob", "value", "valueRaw", "timestamp") VALUES (?, ?, ?,  ?, ?, ?)`,
				BulkArgs: [][]interface{}{
					{"a", nil, "j", "1.000000", int64(4607182418800017408), int64(1000)},
					{"b", "bar", "j", "2.000000", int64(4611686018427387904), int64(1000)},
				},
			},
		},
		{
			series: []*remote.TimeSeries{
				{
					Labels: []*remote.LabelPair{
						&remote.LabelPair{Name: "\"'", Value: "\"'"},
					},
					Samples: []*remote.Sample{
						{Value: 1, TimestampMs: 1000},
					},
				},
			},
			request: &crateRequest{
				Stmt: `INSERT INTO metrics ("l\"\'", "value", "valueRaw", "timestamp") VALUES (?,  ?, ?, ?)`,
				BulkArgs: [][]interface{}{
					{"\"'", "1.000000", int64(4607182418800017408), int64(1000)},
				},
			},
		},
	}

	for _, c := range cases {
		result := writesToCrateRequest(&remote.WriteRequest{Timeseries: c.series})
		require.Equal(t, c.request, result)
	}

}
