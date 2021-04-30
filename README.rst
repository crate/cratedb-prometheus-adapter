==========================
CrateDB Prometheus Adapter
==========================

|version| |ci-tests| |license|

This is an adapter that accepts Prometheus remote read/write requests,
and sends them to CrateDB. This allows using CrateDB as long term storage
for Prometheus.

Along the lines, the program also exports metrics about itself, using the
``cratedb_prometheus_adapter_`` prefix.

Requires CrateDB **3.1.0** or greater.

Building from source
====================

To build the CrateDB Prometheus Adapter from source, you need to have a working
Go environment with **Golang version 1.16** installed.

Use the ``go`` tool to download and install the ``cratedb-prometheus-adapter`` executable
into your ``GOPATH``:

.. code-block:: console

   $ go get github.com/crate/cratedb-prometheus-adapter
   $ cd $GOPATH/src/github.com/crate/cratedb-prometheus-adapter

Alternatively, you can clone the repository and compile the binary:

.. code-block:: console

   $ mkdir -pv ${GOPATH}/src/github.com/crate
   $ cd ${GOPATH}/src/github.com/crate
   $ git clone https://github.com/crate/cratedb-prometheus-adapter.git
   $ cd cratedb-prometheus-adapter
   $ go build

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

  # When using the single binary
  ./cratedb-prometheus-adapter

  # When using Docker
  docker run -it --rm --publish=9268:9268 ghcr.io/crate/cratedb-prometheus-adapter


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
    password: ""              # Password to use (default: "").
    schema: ""                # Schema to use (default: "").
    connect_timeout: 10       # TCP connect timeout (seconds) (default: 10).
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


Building the Docker image
=========================

The project contains a ``Dockerfile`` which can be used to build a Docker
image.

.. code-block:: console

   $ docker build --rm --tag crate/cratedb-prometheus-adapter .

When running the adapter inside Docker, you need to make sure that the running
container has access to the CrateDB instance(s) which it should write to / read
from.

To expose the ``/read``, ``/write`` and ``/metrics`` endpoints, the port
``9268`` must be published.

.. code-block:: console

   $ docker run --rm -ti -p 9268:9268 crate/cratedb-prometheus-adapter

Since the default configuration would use ``localhost`` as CrateDB endpoint, a
``config.yml`` with the correct configuration needs to be mounted on
``/etc/cratedb-prometheus-adapter/config.yml``.

.. code-block:: console

   $ docker run --rm -ti -p 9268:9268 -v $(pwd)/config.yml:/etc/cratedb-prometheus-adapter/config.yaml crate/cratedb-prometheus-adapter

Running with systemd
====================

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
