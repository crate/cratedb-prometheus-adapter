==========================
CrateDB Prometheus Adapter
==========================

This is an adapter that accepts Prometheus remote read/write requests,
and sends them on to CrateDB. This allows using CrateDB as long term storage
for Prometheus.

Requires CrateDB 2.2.0 or greater.

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

By default the adapter will listen on port ``9268``, and talk to the local CrateDB running on port ``4200``.
This is configurable via command line flags, which you can see by passing the ``-h`` flag.

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

Copy `<crate_adapter.service>`_ to ``/etc/systemd/system/crate_adapter.service`` or
just link the service file by running: ``sudo systemctl link $(pwd)/crate_adapter.service``
and run::

  systemctl daemon-reload

Simply configure the adapter by changing the settings in ``/etc/default/crate_adapter``
based on the `<crate_adapter.default>`_ template. After that you can::

  systemctl start crate_adapter
  systemctl enable crate_adapter
