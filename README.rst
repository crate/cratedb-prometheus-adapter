.. highlight:: sh

==========================
CrateDB Prometheus Adapter
==========================

|version| |ci-tests| |license|

The `remote write`_ and `remote read`_ features of Prometheus allow transparently
sending and receiving samples. This is primarily intended for long term storage.

The CrateDB Prometheus Adapter accepts Prometheus remote read/write requests
and sends them to CrateDB. This lets you use CrateDB as long term storage for
Prometheus, OpenTelemetry, and many other system- and application-telemetry
solutions.

Features
========

- Support for the Prometheus remote read and remote write interfaces.

- Support for storing `OpenTelemetry`_ metrics data through
  `OpenTelemetry Collector`_'s `Prometheus Remote Write Exporter`_,
  see documentation about `OpenTelemetry and CrateDB`_.

- The program also exports its own metrics using the
  ``cratedb_prometheus_adapter_`` prefix.

Setup
=====

The CrateDB Prometheus Adapter is offered in two different distribution
variants. You can choose freely which fits your needs best.

- `Native release archives`_ for Linux, Darwin and Windows platforms.
- Public Docker image `ghcr.io/crate/cratedb-prometheus-adapter`_.

.. _Native release archives: https://cdn.crate.io/downloads/dist/prometheus/
.. _ghcr.io/crate/cratedb-prometheus-adapter: https://ghcr.io/crate/cratedb-prometheus-adapter


Usage
=====

Create the following table in your CrateDB database using `ddl.sql`_:

.. code-block:: sql

  CREATE TABLE "metrics" (
      "timestamp" TIMESTAMP,
      "labels_hash" STRING,
      "labels" OBJECT(DYNAMIC),
      "value" DOUBLE,
      "valueRaw" LONG,
      "day__generated" TIMESTAMP GENERATED ALWAYS AS date_trunc('day', "timestamp"),
      PRIMARY KEY ("timestamp", "labels_hash", "day__generated")
    ) PARTITIONED BY ("day__generated");

Depending on data volume and retention you might want to optimize your partitioning scheme
and create hourly, weekly, ... partitions.

Then, run the adapter::

    # When using the single binary
    ./cratedb-prometheus-adapter

    # When using Docker
    docker run -it --rm --publish=9268:9268 ghcr.io/crate/cratedb-prometheus-adapter

By default, the adapter will listen on port ``9268`` and will use a built-in
default configuration. More details how to individually configure it are
outlined within the next section.

To display all available command line options and flags, use the ``-h`` flag.

In order to inquire the ``/metrics`` HTTP endpoint, use an HTTP client like ``curl``::

    curl localhost:9268/metrics

CrateDB adapter endpoint configuration
======================================

To configure the CrateDB endpoint(s) the adapter will write to, the path to the
YAML-based configuration file can be provided by using the ``-config.file``
command line option.

To create a blueprint configuration file, run::

    ./cratedb-prometheus-adapter -config.make > config.yml

If multiple endpoints are listed, the adapter will load-balance between them.
The configuration settings (with one example endpoint) are as below:

.. code-block:: yaml

  cratedb_endpoints:
  - host: "localhost"         # Host to connect to (default: "localhost").
    port: 5432                # Port to connect to (default: 5432).
    user: "crate"             # Username to use (default: "crate")
    password: ""              # Password to use (default: "").
    schema: ""                # Schema to use (default: "").
    max_connections: 0        # The maximum number of concurrent connections (default: runtime.NumCPU()).
                              # It will get forwarded to pgx's `pool_max_conns`, and determines
                              # the maximum number of connections in the connection pool for
                              # both connection pools (read and write).
    read_pool_size_max: 0     # Configure the maximum pool size for read operations individually.
                              # (default: runtime.NumCPU())
    write_pool_size_max: 0    # Configure the maximum pool size for write operations individually.
                              # (default: runtime.NumCPU())
    connect_timeout: 10       # TCP connect timeout (seconds) (default: 10).
                              # It has the same meaning as libpq's `connect_timeout`.
    read_timeout: 5           # Query context timeout for read queries (seconds) (default: 5).
    write_timeout: 5          # Query context timeout for write queries (seconds) (default: 5).
    enable_tls: false         # Whether to connect using TLS (default: false).
    allow_insecure_tls: false # Whether to allow insecure / invalid TLS certificates (default: false).

Timeout Settings
----------------

The unit for all values is *seconds*.

- To adjust the TCP connection timeout, use the ``connect_timeout`` setting.
- To adjust the query timeouts to cancel running operations, use either
  the ``read_timeout`` and ``write_timeout`` settings.

`Soham Kamani <https://github.com/sohamkamani>`_ states it well:

    pgx4 implements query timeouts using context cancellation.

    In production applications, it is *always* preferred to have timeouts for all queries:
    A sudden increase in throughput or a network issue can lead to queries slowing down by
    orders of magnitude.

    Slow queries block the connections that they are running on, preventing other queries
    from running on them. We should always set a timeout after which to cancel a running
    query, to unblock connections in these cases.

    -- `Query Timeouts - Using Context Cancellation`_

Connection Pool Settings
------------------------

The service uses two connection pools for communicating to the database, one of each
for read vs. write operations. The configuration settings ``max_connections``,
``read_pool_size_max``, and ``write_pool_size_max`` determine the maximum
connection pool sizes, either for both pools at once, or individually.

By default, when not configured otherwise, by either omitting the settings altogether,
or using ``0`` values, ``pgx`` configures the maximum pool size using the number of CPU
cores available to the system it is running on, by calling ``runtime.NumCPU()``.


Prometheus configuration
========================

In order to forward write and read requests to the CrateDB adapter, adjust your
``prometheus.yml`` like:

.. code-block:: yaml

  remote_write:
     - url: http://localhost:9268/write
  remote_read:
     - url: http://localhost:9268/read

The adapter also exposes Prometheus metrics on ``/metrics``, which can be scraped in the usual way.


Running as systemd service
==========================

In order to invoke ``cratedb-prometheus-adapter`` as a system service on Linux,
the repository provides corresponding configuration files to deploy the program
as a ``systemd`` service unit. This section outlines how to do this.

For the systemd-based setup, you need four files to be correctly deployed to
your machine.

1. ``/usr/bin/cratedb-prometheus-adapter``.
   This is the program itself, extracted from the corresponding tarball
   distribution package at https://cdn.crate.io/downloads/dist/prometheus/.
2. ``/etc/cratedb-prometheus-adapter/config.yml``.
   Get it from `config.yml`_ and adjust the settings according to your needs.
3. ``/etc/systemd/system/cratedb-prometheus-adapter.service``.
   Get it from `cratedb-prometheus-adapter.service`_.
4. ``/etc/default/cratedb-prometheus-adapter``.
   Get it from `cratedb-prometheus-adapter.default`_.

Mostly, you will only need to make any adjustments to the configuration file
``/etc/cratedb-prometheus-adapter/config.yml``.

After deploying those files correctly, invoking the following commands will
start the service, and enable it to be started automatically on system boot::

    systemctl daemon-reload
    systemctl restart cratedb-prometheus-adapter
    systemctl enable cratedb-prometheus-adapter


.. |version| image:: https://img.shields.io/github/tag/crate/cratedb-prometheus-adapter.svg
    :alt: Version
    :target: https://github.com/crate/cratedb-prometheus-adapter

.. |ci-tests| image:: https://github.com/crate/cratedb-prometheus-adapter/workflows/Tests/badge.svg
    :alt: CI status
    :target: https://github.com/crate/cratedb-prometheus-adapter/actions?workflow=Tests

.. |license| image:: https://img.shields.io/badge/License-Apache%202.0-blue.svg
    :alt: License: Apache 2.0
    :target: https://opensource.org/licenses/Apache-2.0


.. _config.yml: https://github.com/crate/cratedb-prometheus-adapter/blob/main/config.yml
.. _cratedb-prometheus-adapter.default: https://github.com/crate/cratedb-prometheus-adapter/blob/main/systemd/cratedb-prometheus-adapter.default
.. _cratedb-prometheus-adapter.service: https://github.com/crate/cratedb-prometheus-adapter/blob/main/systemd/cratedb-prometheus-adapter.service
.. _ddl.sql: https://github.com/crate/cratedb-prometheus-adapter/blob/main/sql/ddl.sql
.. _OpenTelemetry: https://opentelemetry.io/
.. _OpenTelemetry and CrateDB: https://cratedb.com/docs/guide/integrate/opentelemetry/
.. _OpenTelemetry Collector: https://opentelemetry.io/docs/collector/
.. _Prometheus Remote Write Exporter: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusremotewriteexporter
.. _Query Timeouts - Using Context Cancellation: https://www.sohamkamani.com/golang/sql-database/#query-timeouts---using-context-cancellation
.. _remote read: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_read
.. _remote write: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write
