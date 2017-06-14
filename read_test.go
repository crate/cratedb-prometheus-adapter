package main

import (
	"testing"

	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/require"
)

func TestQueryToSQL(t *testing.T) {
	cases := []struct {
		query *remote.Query
		sql   string
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
			sql: `SELECT * from metrics WHERE ("ln" = 'v') AND ("ln" != 'v') AND ("ln" ~ 'v') AND ("ln" !~ 'v') AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
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
			sql: `SELECT * from metrics WHERE ("ln\"" = 'v\'') AND ("ln\"" != 'v\'') AND ("ln\"" ~ 'v\'') AND ("ln\"" !~ 'v\'') AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
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
			sql: `SELECT * from metrics WHERE ("ln" IS NULL) AND ("ln" IS NOT NULL) AND ("ln" ~ '') AND ("ln" !~ '') AND (timestamp <= 2000) AND (timestamp >= 1000) ORDER BY timestamp`,
		},
	}

	for _, c := range cases {
		require.Equal(t, c.sql, queryToSQL(c.query))
	}

}
