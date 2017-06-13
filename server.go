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

	"github.com/prometheus/prometheus/storage/remote"
)

// Escaping for strings for Crate.io SQL.
var escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

type bulkCrateRequest struct {
	Stmt     string          `json:"stmt"`
	BulkArgs [][]interface{} `json:"bulk_args"`
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
			escapedLabels = append(escapedLabels, fmt.Sprintf("\"l%s\"", escaper.Replace(l)))
		}

		request := bulkCrateRequest{
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

	http.ListenAndServe(":1234", nil)
}
