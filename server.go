package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
)

// Escaping for strings for Crate.io SQL.
var escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "'", "\\'")

// Escape a string for use in SQL.
func escape(s, quote string) string {
	return quote + escaper.Replace(s) + quote
}

type crateRequest struct {
	Stmt     string          `json:"stmt"`
	BulkArgs [][]interface{} `json:"bulk_args,omitempty"`
}

type crateResponse struct {
	Cols []string        `json:"cols,omitempty"`
	Rows [][]interface{} `json:"rows,omitempty"`
}

func runQuery(q *remote.Query) []*remote.TimeSeries {
	resp := []*remote.TimeSeries{}

	selectors := make([]string, 0, len(q.Matchers)+2)
	for _, m := range q.Matchers {
		switch m.Type {
		case remote.MatchType_EQUAL:
			if m.Value == "" {
				// Empty labels are recorded as NULL.
				// In PromQL, empty labels and missing labels are the same thing.
				selectors = append(selectors, fmt.Sprintf("(%s IS NULL)", escape("l"+m.Name, "\"")))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s = %s)", escape("l"+m.Name, "\""), escape(m.Value, "'")))
			}
		case remote.MatchType_NOT_EQUAL:
			if m.Value == "" {
				selectors = append(selectors, fmt.Sprintf("(%s IS NOT NULL)", escape("l"+m.Name, "\"")))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s != %s)", escape("l"+m.Name, "\""), escape(m.Value, "'")))
			}
		case remote.MatchType_REGEX_MATCH:
			// XXX Handle that null == empty string.
			// Crate regexes are not RE2, so there may be small semantic differences here.
			selectors = append(selectors, fmt.Sprintf("(%s ~ %s)", escape("l"+m.Name, "\""), escape(m.Value, "'")))
		case remote.MatchType_REGEX_NO_MATCH:
			selectors = append(selectors, fmt.Sprintf("(%s !~ %s)", escape("l"+m.Name, "\""), escape(m.Value, "'")))
		}
	}
	selectors = append(selectors, fmt.Sprintf("(timestamp <= %d)", q.EndTimestampMs))
	selectors = append(selectors, fmt.Sprintf("(timestamp >= %d)", q.StartTimestampMs))

	query := fmt.Sprintf("SELECT * from metrics WHERE %s ORDER BY timestamp", strings.Join(selectors, " AND "))

	request := crateRequest{Stmt: query}
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return resp
	}

	result, err := http.Post("http://localhost:4200/_sql", "application/json", bytes.NewReader(jsonRequest))
	if err != nil {
		return resp
	}
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		return resp
	}
	var data crateResponse
	decoder := json.NewDecoder(result.Body)
	decoder.UseNumber()
	if err = decoder.Decode(&data); err != nil {
		return resp
	}

	timeseries := map[string]*remote.TimeSeries{}
	for _, row := range data.Rows {
		metric := model.Metric{}
		var v float64
		var t int64
		for i, value := range row {
			column := data.Cols[i]
			if column[0] == 'l' && value != nil {
				metric[model.LabelName(column[1:])] = model.LabelValue(value.(string))
			} else if column == "timestamp" {
				t, _ = value.(json.Number).Int64()
			} else if column == "valueRaw" {
				val, _ := value.(json.Number).Int64()
				v = math.Float64frombits(uint64(val))
			}
		}
		ts, ok := timeseries[metric.String()]
		if !ok {
			ts = &remote.TimeSeries{}
			for k, v := range metric {
				ts.Labels = append(ts.Labels, &remote.LabelPair{Name: string(k), Value: string(v)})
			}
			timeseries[metric.String()] = ts
		}
		ts.Samples = append(ts.Samples, &remote.Sample{Value: v, TimestampMs: t})
	}

	for _, ts := range timeseries {
		resp = append(resp, ts)
	}
	return resp
}

func main() {
	http.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req remote.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Build a list of every label name used.
		labelsUsed := map[string]struct{}{}
		for _, ts := range req.Timeseries {
			for _, l := range ts.Labels {
				labelsUsed[l.Name] = struct{}{}
			}
		}
		labels := make([]string, 0, len(labelsUsed))
		escapedLabels := make([]string, 0, len(labelsUsed))

		for l := range labelsUsed {
			labels = append(labels, l)
			escapedLabels = append(escapedLabels, escape("l"+l, "\""))
		}

		request := crateRequest{
			BulkArgs: make([][]interface{}, 0, len(req.Timeseries)),
		}
		placeholders := strings.Repeat("?, ", len(labels))
		columns := strings.Join(escapedLabels, ", ")
		request.Stmt = fmt.Sprintf("INSERT INTO metrics (%s, \"value\", \"valueRaw\", \"timestamp\") VALUES (%s ?, ?, ?)", columns, placeholders)

		for _, ts := range req.Timeseries {
			metric := make(map[string]string, len(ts.Labels))
			for _, l := range ts.Labels {
				metric[l.Name] = l.Value
			}

			for _, s := range ts.Samples {
				args := make([]interface{}, 0, len(labels)+2)
				for _, l := range labels {
					if metric[l] == "" {
						args = append(args, nil)
					} else {
						args = append(args, metric[l])
					}
				}
				// Convert to string to handle NaN/Inf/-Inf
				args = append(args, fmt.Sprintf("%f", s.Value))
				// Crate.io can't handle full NaN values as required by Prometheus 2.0,
				// so store the raw bits as an int64.
				args = append(args, int64(math.Float64bits(s.Value)))
				args = append(args, s.TimestampMs)

				request.BulkArgs = append(request.BulkArgs, args)
			}
		}

		jsonRequest, err := json.Marshal(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := http.Post("http://localhost:4200/_sql", "application/json", bytes.NewReader(jsonRequest))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp.StatusCode != http.StatusOK {
			respBody, _ := ioutil.ReadAll(resp.Body)
			fmt.Println(string(respBody))
			http.Error(w, string(respBody), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/read", func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req remote.ReadRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Queries) != 1 {
			http.Error(w, "Can only handle one query.", http.StatusBadRequest)
			return
		}

		resp := remote.ReadResponse{
			Results: []*remote.QueryResult{
				{Timeseries: runQuery(req.Queries[0])},
			},
		}
		data, err := proto.Marshal(&resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-protobuf")
		if _, err := w.Write(snappy.Encode(nil, data)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.ListenAndServe(":1234", nil)
}
