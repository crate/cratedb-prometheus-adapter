import os
import shlex
import subprocess
import time

import pytest
from cratedb_toolkit.util import DatabaseAdapter
from prometheus_api_client import PrometheusConnect
from pueblo.util.proc import process


@pytest.fixture
def adapter_metrics():
    """
    The list of metric names exported by CrateDB Prometheus Adapter.
    """
    return [
        "cratedb_prometheus_adapter_read_crate_failed_total",
        "cratedb_prometheus_adapter_read_crate_latency_seconds_bucket",
        "cratedb_prometheus_adapter_read_crate_latency_seconds_sum",
        "cratedb_prometheus_adapter_read_crate_latency_seconds_count",
        "cratedb_prometheus_adapter_read_failed_total",
        "cratedb_prometheus_adapter_read_latency_seconds_bucket",
        "cratedb_prometheus_adapter_read_latency_seconds_sum",
        "cratedb_prometheus_adapter_read_latency_seconds_count",
        "cratedb_prometheus_adapter_read_timeseries_samples_sum",
        "cratedb_prometheus_adapter_read_timeseries_samples_count",
        "cratedb_prometheus_adapter_write_crate_failed_total",
        "cratedb_prometheus_adapter_write_crate_latency_seconds_bucket",
        "cratedb_prometheus_adapter_write_crate_latency_seconds_sum",
        "cratedb_prometheus_adapter_write_crate_latency_seconds_count",
        "cratedb_prometheus_adapter_write_failed_total",
        "cratedb_prometheus_adapter_write_latency_seconds_bucket",
        "cratedb_prometheus_adapter_write_latency_seconds_sum",
        "cratedb_prometheus_adapter_write_latency_seconds_count",
        "cratedb_prometheus_adapter_write_timeseries_samples_sum",
        "cratedb_prometheus_adapter_write_timeseries_samples_count",
    ]


@pytest.fixture(scope="session")
def reset_database(cratedb_client):
    """
    Create a blank canvas `metrics` database table.
    """
    cratedb_client.run_sql("DROP TABLE IF EXISTS metrics;")
    cratedb_client.run_sql(open("sql/ddl.sql").read())


@pytest.fixture(scope="session")
def flush_database(cratedb_client):
    """
    Let the data converge between background daemons, and flush the database.
    """

    def flush():
        time.sleep(1)
        cratedb_client.run_sql("REFRESH TABLE metrics;")

    return flush


@pytest.fixture(scope="session")
def cratedb_client():
    """
    Provide a database client to the test cases.
    """
    return DatabaseAdapter(dburi=os.environ["CRATEDB_CONNECTION_STRING"])


@pytest.fixture(scope="session")
def prometheus_client():
    """
    Provide a Prometheus client to the test cases.
    """
    return PrometheusConnect(url=os.environ["PROMETHEUS_URL"], disable_ssl=True)


@pytest.fixture(scope="session", autouse=True)
def cratedb_prometheus_adapter(reset_database, cratedb_client, flush_database):
    """
    Start the CrateDB Prometheus Adapter.
    """

    command = "go run ."
    with process(shlex.split(command), stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as daemon:
        # Give the server time to start.
        time.sleep(2)

        # Check if the server started successfully.
        assert not daemon.poll(), daemon.stdout.read().decode("utf-8")
        time.sleep(1)

        # Let the adapter collect a few metrics.
        flush_database()

        yield daemon
