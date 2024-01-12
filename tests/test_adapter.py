import os
from unittest import mock

import pytest
import requests


def test_exported_metrics_http(adapter_metrics):
    """
    Verify the names of the metrics exported by CrateDB Prometheus Adapter.
    Here, the CrateDB Prometheus Adapter is queried using HTTP.
    """
    response = requests.get(os.environ["CRATEDB_PROMETHEUS_ADAPTER_METRICS_URL"])
    assert response.status_code == 200
    assert response.headers["Content-Type"].startswith("text/plain")

    payload = response.text
    for attribute in adapter_metrics:
        assert attribute in payload


def test_all_metrics(prometheus_client, adapter_metrics):
    """
    Verify the list of all the metrics that Prometheus scrapes includes the adapter metrics.
    """
    metrics = sorted(prometheus_client.all_metrics())

    # Verify if all the metrics of the adapter are present.
    for attribute in adapter_metrics:
        assert attribute in metrics


def test_custom_query(prometheus_client):
    """
    Probe the `custom_query` method.
    """

    value = prometheus_client.custom_query(
        query="sum(cratedb_prometheus_adapter_read_crate_failed_total)"
    )
    assert len(value) == 1
    timestamp = mock.ANY
    total_sum = "0"
    assert value[0] == {"metric": {}, "value": [timestamp, total_sum]}


def test_get_metric_aggregation_success(prometheus_client):
    """
    Probe an aggregation inquiry.
    """
    result = prometheus_client.get_metric_aggregation(
        "cratedb_prometheus_adapter_read_crate_failed_total", operations=["max"]
    )
    assert result["max"] == 0


def test_get_metric_aggregation_unknown_metric(prometheus_client):
    """
    Probe inquiring an aggregation on an unknown metric.
    """
    result = prometheus_client.get_metric_aggregation("foobar", operations=["max"])
    assert result is None


def test_get_metric_aggregation_unknown_operation(prometheus_client):
    """
    Probe inquiring an aggregation using an unknown operation.
    """
    with pytest.raises(TypeError) as ex:
        prometheus_client.get_metric_aggregation(
            "cratedb_prometheus_adapter_write_timeseries_samples_count", operations=["unknown"]
        )
    assert ex.match("Invalid operation: unknown")


def test_verify_no_failures(prometheus_client):
    """
    Verify no failures happened by inspecting corresponding metrics.
    """
    failed_total_attributes = [
        "cratedb_prometheus_adapter_read_crate_failed_total",
        "cratedb_prometheus_adapter_read_failed_total",
        "cratedb_prometheus_adapter_write_crate_failed_total",
        "cratedb_prometheus_adapter_write_failed_total",
    ]
    for failed_total_attribute in failed_total_attributes:
        result = prometheus_client.get_metric_aggregation(
            failed_total_attribute, operations=["sum"]
        )
        assert result == {"sum": 0.0}


def test_verify_write_activity(prometheus_client):
    """
    Verify the write-path works well.
    """
    result = prometheus_client.get_current_metric_value(
        metric_name="cratedb_prometheus_adapter_write_timeseries_samples_sum"
    )
    value = int(result[0]["value"][1])
    assert value > 1_000


def test_verify_read_activity(prometheus_client, flush_database):
    """
    Verify the read-path works well.
    """

    # 1. Submit a few read requests.
    query_metrics = [
        "cratedb_prometheus_adapter_write_latency_seconds_count",
        "cratedb_prometheus_adapter_write_latency_seconds_bucket",
        "cratedb_prometheus_adapter_read_failed_total",
        "cratedb_prometheus_adapter_write_failed_total",
        "prometheus_http_requests_total",
        "up",
    ]
    for metric in query_metrics:
        prometheus_client.custom_query(query=metric)

    # Wait for telemetry data to settle.
    flush_database()

    # 2. Verify data arrived in database.
    result = prometheus_client.get_current_metric_value(
        metric_name="cratedb_prometheus_adapter_read_timeseries_samples_sum"
    )
    value = int(result[0]["value"][1])
    assert value >= 6
