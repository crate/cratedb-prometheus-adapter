"""
OpenTelemetry probe application.

opentelemetry-instrument --logs_exporter=otlp --service_name=app python tests/otel_probe.py

- https://opentelemetry.io/docs/languages/python/getting-started/
- https://opentelemetry.io/docs/languages/python/instrumentation/
- https://github.com/open-telemetry/opentelemetry-python/discussions/3192
"""
from opentelemetry import metrics
from opentelemetry.sdk.metrics import MeterProvider


def main():

    provider = MeterProvider()
    metrics.set_meter_provider(provider)

    meter = metrics.get_meter("testdrive.meter.name")
    temperature = meter.create_gauge("temperature")
    humidity = meter.create_gauge("humidity")

    temperature.set(42.42)
    humidity.set(84.84)

    provider.force_flush()


if __name__ == "__main__":
    main()
