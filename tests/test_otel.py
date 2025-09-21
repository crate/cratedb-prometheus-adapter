"""
Verify OpenTelemetry probe.
"""
import os
import sys


def test_opentelemetry(cratedb_client, flush_database):

    # Invoke application probe that sends metrics to the OpenTelemetry Collector.
    os.system(f"opentelemetry-instrument --logs_exporter=otlp --service_name=app {sys.executable} tests/otel_probe.py")
    flush_database()

    # Query database.
    result = cratedb_client.run_sql(
        "SELECT * FROM testdrive.metrics "
        "WHERE labels['__name__'] = 'temperature';", records=True)

    # Verify data in database.
    assert result[0]["labels"] == {
        '__name__': 'temperature',
        'job': 'app',
        'subsystem': 'otel-testdrive',
    }
    assert result[0]["value"] == 42.42
    assert len(result) == 1
