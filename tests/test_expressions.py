"""
## About
Integration tests to verify query expressions end-to-end.

## Resources
- https://pypi.org/project/prometheus-api-client/
- https://prometheus.io/docs/prometheus/latest/querying/examples/
- https://github.com/crate/cratedb-prometheus-adapter/blob/0.5.0/server.go#L124-L160
"""


def test_expression_plain(prometheus_client):
    """
    Verify a plain metrics query, without any label constraints.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
    }
    """)
    assert len(result) > 15


def test_expression_eq_success(prometheus_client):
    """
    Verify a metrics query, with a label constraint using an equality match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job = "prometheus"
    }
    """)
    assert len(result) > 15


def test_expression_eq_empty(prometheus_client):
    """
    Verify a metrics query, with a label constraint using an equality match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job = "foobar"
    }
    """)
    assert len(result) == 0


def test_expression_neq_success(prometheus_client):
    """
    Verify a metrics query, with a label constraint using an inequality match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job != "foobar"
    }
    """)
    assert len(result) > 15


def test_expression_neq_empty(prometheus_client):
    """
    Verify a metrics query, with a label constraint using an inequality match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job != "prometheus"
    }
    """)
    assert len(result) == 0


def test_expression_re_success_basic(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job =~ "prom.+"
    }
    """)
    assert len(result) > 15


def test_expression_re_success_dots(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        code =~ "2.."
    }
    """)
    assert len(result) > 15


def test_expression_re_success_or(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        code =~ "2..|3.."
    }
    """)
    assert len(result) > 15


def test_expression_re_empty(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job =~ "foobar.+"
    }
    """)
    assert len(result) == 0


def test_expression_nre_success_basic(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a negated regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job !~ "foobar.+"
    }
    """)
    assert len(result) > 15


def test_expression_nre_success_dots(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a negated regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        code !~ "5.."
    }
    """)
    assert len(result) > 15


def test_expression_nre_empty(prometheus_client):
    """
    Verify a metrics query, with a label constraint using a negated regular expression match.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        job !~ "prom.+"
    }
    """)
    assert len(result) == 0


def test_expression_all(prometheus_client):
    """
    Verify a metrics query, with label constraints using all types of label matchers.
    """
    result = prometheus_client.custom_query(query="""
    prometheus_http_requests_total{
        code = "200",
        handler != "foobar",
        job =~ "prom.+",
        instance !~ "foobar",
    }
    """)
    assert len(result) > 15


def test_range_vector_selector_success(prometheus_client):
    """
    Verify range vector literals.

    https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors
    """
    result = prometheus_client.custom_query(query="""
    cratedb_prometheus_adapter_write_timeseries_samples_count{}[10s]
    """)
    assert len(result) == 1


def test_range_vector_selector_empty(prometheus_client):
    """
    Verify range vector literals. With a minimal low value, it will produce zero results.

    https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors
    """
    result = prometheus_client.custom_query(query="""
    cratedb_prometheus_adapter_write_timeseries_samples_count{}[1ms]
    """)
    assert len(result) == 0
