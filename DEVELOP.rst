===============
Developer Guide
===============


Building from source
====================

To build the CrateDB Prometheus Adapter from source, you need to have a working
Go environment with **Golang version 1.16** installed.

Use the ``go`` tool to download and install the ``cratedb-prometheus-adapter`` executable
into your ``GOPATH``:

.. code-block:: console

   go get github.com/crate/cratedb-prometheus-adapter
   cd $GOPATH/src/github.com/crate/cratedb-prometheus-adapter

Alternatively, you can clone the repository and compile the binary:

.. code-block:: console

   mkdir -pv ${GOPATH}/src/github.com/crate
   cd ${GOPATH}/src/github.com/crate
   git clone https://github.com/crate/cratedb-prometheus-adapter.git
   cd cratedb-prometheus-adapter
   go build


Setup
=====

To start things off, clone the repository and change into the newly checked out
directory:

.. code-block:: console

   mkdir -pv ${GOPATH}/src/github.com/crate
   cd ${GOPATH}/src/github.com/crate
   git clone https://github.com/crate/cratedb-prometheus-adapter.git
   cd cratedb-prometheus-adapter

To simply run the adapter, invoke:

.. code-block:: console

   go run .

To build the ``cratedb-prometheus-adapter`` executable, run:

.. code-block:: console

   go build

To run the test suite, execute:

.. code-block:: console

   go test


Preparing a Release
===================

To create a new release, you must:

- Make sure all fixes are backported to the current stable branch ``x.y``
  (e.g. ``1.1``)

- For new feature releases, create a new stable branch ``x.(y+1)``
  (e.g. ``1.2``)

On the release branch:

- Update the ``version`` constant in ``server.go``

- Add a section for the new version in the ``CHANGES.rst`` file

- Commit your changes with a message like "prepare release x.y.z"

- Push to origin/<release_branch>

- Create a tag by running ``./devtools/create_tag.sh``

On branch "main":

- Update the release notes to reflect the release

- Update the ``version`` constant to the next minor version and suffix it with
  ``-dev`` (e.g. ``1.3.0-dev``)

Next:

- Trigger the build/release script on `Jenkins CI`_ for the newly created tag

.. _Jenkins CI: https://jenkins.crate.io


Building the Docker image
=========================

The project contains a ``Dockerfile`` which can be used to build a Docker
image.

.. code-block:: console

   docker build --rm --tag crate/cratedb-prometheus-adapter .

When running the adapter inside Docker, you need to make sure that the running
container has access to the CrateDB instance(s) which it should write to / read
from.

To expose the ``/read``, ``/write`` and ``/metrics`` endpoints, the port
``9268`` must be published.

.. code-block:: console

   docker run --rm -ti -p 9268:9268 crate/cratedb-prometheus-adapter

Since the default configuration would use ``localhost`` as CrateDB endpoint, a
``config.yml`` with the correct configuration needs to be mounted on
``/etc/cratedb-prometheus-adapter/config.yml``.

.. code-block:: console

   docker run --rm -ti -p 9268:9268 -v $(pwd)/config.yml:/etc/cratedb-prometheus-adapter/config.yaml crate/cratedb-prometheus-adapter
