package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
)

// Escaping for strings for Crate.io SQL.
var escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "'", "\\'")

// Escape a labelname for use in SQL as a column name.
func escapeLabelName(s string) string {
	return "\"l" + escaper.Replace(s) + "\""
}

// Escape a labelvalue for use in SQL as a string value.
func escapeLabelValue(s string) string {
	return "'" + escaper.Replace(s) + "'"
}

type crateRequest struct {
	Stmt     string          `json:"stmt"`
	BulkArgs [][]interface{} `json:"bulk_args,omitempty"`
}

type crateResponse struct {
	Cols []string        `json:"cols,omitempty"`
	Rows [][]interface{} `json:"rows,omitempty"`
}

// Convert a read query into a Crate SQL query.
func queryToSQL(q *remote.Query) (string, error) {
	selectors := make([]string, 0, len(q.Matchers)+2)
	for _, m := range q.Matchers {
		switch m.Type {
		case remote.MatchType_EQUAL:
			if m.Value == "" {
				// Empty labels are recorded as NULL.
				// In PromQL, empty labels and missing labels are the same thing.
				selectors = append(selectors, fmt.Sprintf("(%s IS NULL)", escapeLabelName(m.Name)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s = %s)", escapeLabelName(m.Name), escapeLabelValue(m.Value)))
			}
		case remote.MatchType_NOT_EQUAL:
			if m.Value == "" {
				selectors = append(selectors, fmt.Sprintf("(%s IS NOT NULL)", escapeLabelName(m.Name)))
			} else {
				selectors = append(selectors, fmt.Sprintf("(%s != %s)", escapeLabelName(m.Name), escapeLabelValue(m.Value)))
			}
		case remote.MatchType_REGEX_MATCH:
			re := "^(?:" + m.Value + ")$"
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
		case remote.MatchType_REGEX_NO_MATCH:
			re := "^(?:" + m.Value + ")$"
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

	return fmt.Sprintf("SELECT * from metrics WHERE %s ORDER BY timestamp", strings.Join(selectors, " AND ")), nil
}

func responseToTimeseries(data *crateResponse) []*remote.TimeSeries {
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
			labelnames := make([]string, 0, len(metric))
			for k := range metric {
				labelnames = append(labelnames, string(k))
			}
			sort.Strings(labelnames) // Sort for unittests.
			for _, k := range labelnames {
				ts.Labels = append(ts.Labels, &remote.LabelPair{Name: string(k), Value: string(metric[model.LabelName(k)])})
			}
			timeseries[metric.String()] = ts
		}
		ts.Samples = append(ts.Samples, &remote.Sample{Value: v, TimestampMs: t})
	}

	names := make([]string, 0, len(timeseries))
	for k := range timeseries {
		names = append(names, k)
	}
	sort.Strings(names)
	resp := make([]*remote.TimeSeries, 0, len(timeseries))
	for _, name := range names {
		resp = append(resp, timeseries[name])
	}
	return resp
}

type crateAdapter struct {
	client http.Client
	url    string
}

func (ca *crateAdapter) runQuery(q *remote.Query) ([]*remote.TimeSeries, error) {
	query, err := queryToSQL(q)
	if err != nil {
		return nil, err
	}

	request := crateRequest{Stmt: query}
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	result, err := ca.client.Post(ca.url, "application/json", bytes.NewReader(jsonRequest))
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		return nil, err
	}
	var data crateResponse
	decoder := json.NewDecoder(result.Body)
	decoder.UseNumber()
	if err = decoder.Decode(&data); err != nil {
		return nil, err
	}

	timeseries := responseToTimeseries(&data)
	return timeseries, nil
}

func (ca *crateAdapter) handleRead(w http.ResponseWriter, r *http.Request) {
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

	result, err := ca.runQuery(req.Queries[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := remote.ReadResponse{
		Results: []*remote.QueryResult{
			{Timeseries: result},
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
}

func (ca *crateAdapter) handleWrite(w http.ResponseWriter, r *http.Request) {
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
		escapedLabels = append(escapedLabels, escapeLabelName(l))
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

	resp, err := ca.client.Post(ca.url, "application/json", bytes.NewReader(jsonRequest))
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
}

func main() {
	ca := crateAdapter{
		client: http.Client{},
		url:    "http://localhost:4200/_sql",
	}

	http.HandleFunc("/write", ca.handleWrite)
	http.HandleFunc("/read", ca.handleRead)
	http.ListenAndServe(":1234", nil)
}
