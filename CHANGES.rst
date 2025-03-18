======================================
CHANGES for CrateDB Prometheus Adapter
======================================

Unreleased
==========
- Updated `prometheus/common` from 0.62.0 to 0.63.0
- OCI: Updated images to use Golang 1.24

2025-03-05 0.5.4
================
- Updated `prometheus/client_golang` from 1.21.0 to 1.21.1,
  addressing a performance regression introduced in `PROMETHEUS-CLIENT-1661`_.

.. _PROMETHEUS-CLIENT-1661: https://github.com/prometheus/client_golang/pull/1661

2025-02-27 0.5.3
================
- Removed stray creation of new connection pool on every request,
  likely resolving the memory leak introduced with v0.5.0.
  Thanks, @widmogrod.
- Updated Prometheus libraries to
  prometheus/prometheus v0.302.1 and prometheus/common v0.62.0.

2024-11-11 0.5.2
================
- Changed the behavior to ignore samples with identical timestamps but different
  values during ingestion. This change aligns with Prometheus' behavior, as
  duplicates should not occur. However, since this enforcement is recent,
  there may be setups with existing duplicates; thus, we now ignore them to
  prevent errors.
- Update Go images to 1.23.
- Update all deps to latest (prometheus v0.55.1).

2024-01-23 0.5.1
================

- Packaging: Re-add compatibility with glibc 2.31,
  by building on ``golang:1.20-bullseye``.
- Fixed use of ``schema`` configuration setting, it was not honored.


2024-01-12 0.5.0
================

CHANGES
-------
- Accept invoking the program without default configuration file ``config.yml``
  In this case, the program will fall back to the builtin defaults, essentially
  connecting to ``localhost:5432`` with username ``crate``.
- Add query timeouts using context cancellation. The corresponding
  configuration settings are ``read_timeout`` and ``write_timeout``.
- Use a different connection pool for read vs. write operations.
  The corresponding settings to configure the maximum pool sizes
  are ``read_pool_size_max`` and ``write_pool_size_max``.
- Accept invocation without default configuration file ``config.yml``.
- Add command line option ``-config.make`` to print a blueprint configuration
  file to stdout.
- Use a DSN-style connection string for talking to pgx5.
- Add program version to startup log message.

DEPENDENCIES
------------
- Add support for Go 1.20 and 1.21, drop support for previous releases
- Update dependency packages across the board to their latest or minor patch releases
- Update Prometheus libraries (client: 1.18, server: 2.48)
- Update Protocol Buffers libraries (google.golang.org/protobuf 1.31)
- Update to pgx5 library

BREAKING CHANGES
----------------
- This release removes the default value for the ``-config.file`` command line
  option, which was ``config.yml``. When the option is omitted, the service
  will use the built-in settings, connecting to CrateDB on ``localhost:5432``.


2021-05-04 0.4.0
================

BREAKING CHANGES
----------------

- This release changes the toplevel configuration section name to ``cratedb_endpoints``.
  It is an aftermath of the "naming things" refactorings happening in 0.3.0.

CHANGES
-------

- Improve network behaviour: Adjust TCP timeout and keepalive settings to
  mitigate problems that can occur when the adapter in connecting to CrateDB
  via a load balancer that may drop idle connections in-transparently, such as
  in AKS. The default values are:

    - KeepAlive: 30 seconds
    - ConnectTimeout: 10 seconds

  The TCP connect timeout can be adjusted by using the ``-tcp.connect.timeout``
  option.

2021-04-29 0.3.0
================

BREAKING CHANGES
----------------

- This release changes the program name to ``cratedb-prometheus-adapter``
  and the default prefix for exported metrics to ``cratedb_prometheus_adapter_``.
  The latter can be reconfigured using the new ``-metrics.export.prefix`` option.

CHANGES
-------

- Provide a default ``config.yml`` in the Docker image, which can be replaced
  by mounting a file on ``/etc/cratedb-prometheus-adapter/config.yml``.

- Made Go 1.16 a minimum requirement.

- Updated project to make use of `Go modules <https://golang.org/ref/mod>`_
  instead of Govendor.

- Renamed the program to ``cratedb-prometheus-adapter``.

- Renamed the exported metric prefix to ``cratedb_prometheus_adapter_``. It is
  now, for example, ``cratedb_prometheus_adapter_write_latency_seconds``.
  Attention: This is a breaking change with respect to your exported metric
  names. In order to keep the former name, use
  ``./cratedb-prometheus-adapter -metrics.export.prefix=crate_adapter_``.

2019-03-06 0.2.1
================

- Fixed the translation of prometheus queries using regular expressions
  (``metric_name{job=~"something"}``) , so that the generated SQL queries match
  the proper records in CrateDB.

- Fixed an issue that caused reads to increment the write metrics instead of
  the read metrics.

2018-07-10 0.2.0
================

- Use Postgres wire protocol (pgx client library) to connect to CrateDB:

  - This change requires CrateDB 3.1.0 or newer!

  - Connections can be configured via ``crate.yml`` configuration file using
    the ``-config.file`` flag.

  - Added support for multiple endpoints.

2017-06-11 0.1
==============

- Unofficial experimental release
