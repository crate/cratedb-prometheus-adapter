==========================
CrateDB Prometheus Adapter
==========================

|version| |ci-tests| |license|

This is an adapter that accepts Prometheus remote read/write requests,
and sends them to CrateDB. This allows using CrateDB as long term storage
for Prometheus.

Along the lines, the program also exports metrics about itself, using the
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

Create the following table in your CrateDB database:

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

Then run the adapter::

    # When using the single binary
    ./cratedb-prometheus-adapter

    # When using Docker
    docker run -it --rm --publish=9268:9268 ghcr.io/crate/cratedb-prometheus-adapter

By default the adapter will listen on port ``9268`` and will use a built-in
configuration as outlined in the next section.
This and more is configurable via command line flags, which you can see by
passing the ``-h`` flag.

The CrateDB endpoints are provided in a configuration file, which defaults to
``config.yml`` (``-config.file`` flag). The included example configuration file
forwards samples to a CrateDB running on ``localhost`` on port ``5432``.

CrateDB Adapter Endpoint Configuration
======================================

The path to the YAML-based configuration file can be provided by using the
``-config.file`` command line option.
The settings describe the CrateDB endpoints the adapter will write to.

If multiple endpoints are listed, the adapter will load-balance between them.
The options (with one example endpoint) are as below:

.. code-block:: yaml

  cratedb_endpoints:
  - host: "localhost"         # Host to connect to (default: "localhost").
    port: 5432                # Port to connect to (default: 5432).
    user: "crate"             # Username to use (default: "crate")
    password: ""              # Password to use (default: "").
    schema: ""                # Schema to use (default: "").
    connect_timeout: 10       # TCP connect timeout (seconds) (default: 10).
    max_connections: 5        # The maximum number of concurrent connections (default: 5).
    enable_tls: false         # Whether to connect using TLS (default: false).
    allow_insecure_tls: false # Whether to allow insecure / invalid TLS certificates (default: false).

Prometheus Configuration
========================

In order to forward write and read requests to the CrateDB adapter, adjust your
``prometheus.yml`` like:

.. code-block:: yaml

  remote_write:
     - url: http://localhost:9268/write
  remote_read:
     - url: http://localhost:9268/read

The adapter also exposes Prometheus metrics on ``/metrics``, which can be scraped in the usual way.


Running as service
==================

In order to run ``cratedb-prometheus-adapter`` as a system service on Linux,
the repository provides configuration files to configure the program as a
``systemd`` service unit. This section outlines how to apply that configuration.

Copy `<config.yml>`_ to ``/etc/cratedb-prometheus-adapter/config.yml`` and adjust as needed.

Copy `<systemd/cratedb-prometheus-adapter.service>`_ to ``/etc/systemd/system/cratedb-prometheus-adapter.service`` or
just link the service file by running: ``sudo systemctl link $(pwd)/cratedb-prometheus-adapter.service``
and run::

    systemctl daemon-reload

Change flag-based configuration by changing the settings in ``/etc/default/cratedb-prometheus-adapter``
based on the `<systemd/cratedb-prometheus-adapter.default>`_ template. After that you can::

    systemctl start cratedb-prometheus-adapter
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
