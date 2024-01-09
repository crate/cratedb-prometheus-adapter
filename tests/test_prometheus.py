import datetime as dt
from pprint import pprint
from unittest import mock

import pandas as pd
from prometheus_api_client import MetricRangeDataFrame, MetricSnapshotDataFrame


def test_all_metrics(prometheus_client):
    """
    Probe the list of all the metrics that the Prometheus host scrapes.
    """
    metrics = sorted(prometheus_client.all_metrics())
    pprint(metrics)
    assert len(metrics) > 250
    assert "go_gc_duration_seconds" in metrics
    assert "up" in metrics


def test_get_value_success(prometheus_client):
    """
    Probe the `get_current_metric_value` method.
    """
    value = prometheus_client.get_current_metric_value(
        metric_name="up", label_config={"job": "prometheus"}
    )
    assert value == [
        {
            "metric": {"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
            "value": [mock.ANY, "1"],
        }
    ]

    value = prometheus_client.get_current_metric_value(
        metric_name="up", label_config={"job": "cratedb"}
    )
    assert value == [
        {
            "metric": {"__name__": "up", "instance": "host.docker.internal:9268", "job": "cratedb"},
            "value": [mock.ANY, "1"],
        }
    ]


def test_get_value_unknown_metric(prometheus_client):
    """
    Probe the `get_current_metric_value` method with an unknown metric.
    """
    value = prometheus_client.get_current_metric_value(
        metric_name="unknown", label_config={"job": "prometheus"}
    )
    assert value == []


def test_get_value_unknown_label(prometheus_client):
    """
    Probe the `get_current_metric_value` method with an unknown label constraint.
    """

    value = prometheus_client.get_current_metric_value(
        metric_name="up", label_config={"job": "unknown"}
    )
    assert value == []


def test_get_value_any(prometheus_client):
    """
    Without any constraint, verify querying a common metric is included into _two_ series.
    """
    value = prometheus_client.get_current_metric_value(metric_name="up")
    assert len(value) == 2


def test_custom_query(prometheus_client):
    """
    Probe the `custom_query` method.
    """

    # Here, we are fetching the values of a particular metric name
    value = prometheus_client.custom_query(query="prometheus_http_requests_total")
    assert len(value) > 2
    assert value[0] == {
        "metric": {
            "__name__": "prometheus_http_requests_total",
            "code": "200",
            "handler": "/",
            "instance": "localhost:9090",
            "job": "prometheus",
        },
        "value": [mock.ANY, "0"],
    }

    # Now, lets try to fetch the `sum` of the metrics
    value = prometheus_client.custom_query(query="sum(prometheus_http_requests_total)")
    assert len(value) == 1
    timestamp = mock.ANY
    total_sum = mock.ANY
    assert value[0] == {"metric": {}, "value": [timestamp, total_sum]}


def test_query_dataframe_snapshot(prometheus_client):
    """
    Probe the `get_current_metric_value` method, together with `MetricSnapshotDataFrame`.
    """

    metric_data = prometheus_client.get_current_metric_value(metric_name="up")
    metric_df = MetricSnapshotDataFrame(metric_data)
    assert isinstance(metric_df, pd.DataFrame)
    assert list(metric_df.columns) == ["__name__", "instance", "job", "timestamp", "value"]
    assert metric_df.index.name is None
    assert isinstance(metric_df.index[0], int)


def test_query_dataframe_timerange(prometheus_client):
    """
    Probe the `get_metric_range_data` method, together with `MetricRangeDataFrame`.
    """

    metric_data = prometheus_client.get_metric_range_data(
        metric_name="up",
        start_time=(dt.datetime.now() - dt.timedelta(minutes=30)),
        end_time=dt.datetime.now(),
    )
    metric_df = MetricRangeDataFrame(metric_data)
    assert isinstance(metric_df, pd.DataFrame)
    assert list(metric_df.columns) == ["__name__", "instance", "job", "value"]
    assert metric_df.index.name == "timestamp"
    assert isinstance(metric_df.index[0], pd.Timestamp)


def test_query_timerange_future(prometheus_client):
    """
    When querying data from the future, the result is empty.
    """

    metric_data = prometheus_client.get_metric_range_data(
        metric_name="up",
        start_time=dt.datetime.now() + dt.timedelta(minutes=5),
        end_time=dt.datetime.now() + dt.timedelta(minutes=10),
    )
    assert metric_data == []


def test_query_timerange_past(prometheus_client):
    """
    When querying data from the near past, the result is empty.
    """

    metric_data = prometheus_client.get_metric_range_data(
        metric_name="up",
        start_time=dt.datetime.now() - dt.timedelta(days=1095),
        end_time=dt.datetime.now() - dt.timedelta(days=365),
    )
    assert metric_data == []
