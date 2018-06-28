==========================
CrateDB Prometheus Adapter
==========================

This is an adapter that accepts Prometheus remote read/write requests,
and sends them on to CrateDB. This allows using CrateDB as long term storage
for Prometheus.

Requires CrateDB 3.1.0 or greater.

Building
========

::

  go get github.com/crate/crate_adapter
  cd ${GOPATH-$HOME/go}/src/github.com/crate/crate_adapter
  go build

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
and create hourly, weekly,... partitions.

Then run the adapter::

  ./crate_adapter

By default the adapter will listen on port ``9268``.
This and more is configurable via command line flags, which you can see by passing the ``-h`` flag.

The CrateDB endpoints are provided in a configuration file, which defaults to
``config.yml`` (``-config.file`` flag). The included example configuration file forwards
samples to a CrateDB running on ``localhost`` on port ``5432``.

Adapter Crate Endpoint Configuration
====================================

The CrateDB endpoints that the adapter writes to are configured in a YAML-based configuration
file provided by the ``-config.file`` flag. If multiple endpoints are listed, the adapter will
load-balance between them. The options (for one example endpoint) are as below:

.. code-block:: yaml

  crate_endpoints:
  - host: "localhost"         # Host to connect to (default: "localhost").
    port: 5432                # Port to connect to (default: 5432).
    user: "crate"             # Username to use (default: "crate")
    password: "mypass"        # Password to use (default: "").
    schema: "prometheus"      # Schema to use (default: "").
    max_connections: 5        # The maximum number of concurrent connections (default: 5).
    enable_tls: false         # Whether to connect using TLS (default: false).
    allow_insecure_tls: false # Whether to allow insecure / invalid TLS certificates (default: false).

Prometheus Configuration
========================

Add the following to your ``prometheus.yml``:

.. code-block:: yaml

  remote_write:
     - url: http://localhost:9268/write
  remote_read:
     - url: http://localhost:9268/read

The adapter also exposes Prometheus metrics on ``/metrics``, and can be scraped in the usual fashion.

Running with systemd
====================

Copy `<config.yml>`_ to ``/etc/crate_adapter/config.yml`` and adjust as needed.

Copy `<crate_adapter.service>`_ to ``/etc/systemd/system/crate_adapter.service`` or
just link the service file by running: ``sudo systemctl link $(pwd)/crate_adapter.service``
and run::

  systemctl daemon-reload

Change flag-based configuration by changing the settings in ``/etc/default/crate_adapter``
based on the `<crate_adapter.default>`_ template. After that you can::

  systemctl start crate_adapter
  systemctl enable crate_adapter
