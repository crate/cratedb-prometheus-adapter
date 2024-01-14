"""
## About
Submit Prometheus query expressions and display results.
Similar to `promtool query`.

## Setup
```
python3 -m venv .venv
source .venv/bin/activate
pip install --upgrade prometheus-api-client
```

## Synopsis
```
python sandbox/promquery.py 'foobar'
python sandbox/promquery.py 'prometheus_http_requests_total'
python sandbox/promquery.py 'prometheus_http_requests_total{code!="200"}'
python sandbox/promquery.py 'prometheus_http_requests_total{code!~"2.."}'
python sandbox/promquery.py 'rate(prometheus_http_requests_total[5m])[30m:1m]'
```

## Resources
- https://pypi.org/project/prometheus-api-client/
- https://prometheus.io/docs/prometheus/latest/querying/examples/
- https://github.com/crate/cratedb-prometheus-adapter/blob/0.5.0/server.go#L124-L160
"""
import sys
from pprint import pprint

from prometheus_api_client import (
    PrometheusConnect,
)


class PrometheusAdapter:

    def __init__(self, url: str, disable_ssl: bool = False):
        self.url = url
        self.disable_ssl = disable_ssl
        self.prometheus: PrometheusConnect

    def connect(self):
        self.prometheus = PrometheusConnect(url=self.url, disable_ssl=self.disable_ssl)
        return self

    def query(self, expression: str):
        # Default timeout is 5 seconds. Perfect.
        return self.prometheus.custom_query(query=expression)


def run_query(url: str, expression: str):
    prom = PrometheusAdapter(url=url, disable_ssl=True).connect()
    metrics = prom.query(expression=expression)
    pprint(metrics)


if __name__ == "__main__":
    url = "http://localhost:9090/"
    expression = sys.argv[1]
    run_query(url, expression)
