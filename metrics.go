package main

import (
	"fmt"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	writeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%swrite_latency_seconds", *metricsExportPrefix),
		Help: "How long it took us to respond to write requests.",
	})
	writeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%swrite_failed_total", *metricsExportPrefix),
		Help: "How many write request we returned errors for.",
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
		Help: "How long it took us to respond to read requests.",
	})
	readErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%sread_failed_total", *metricsExportPrefix),
		Help: "How many read requests we returned errors for.",
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

func setupMetrics() {
	level.Info(logger).Log("msg", "Exporting internal metrics")
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
