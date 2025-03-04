.. highlight:: bash


===============
Developer Guide
===============


Building from source
====================

To build the CrateDB Prometheus Adapter from source, you need to have a working
Go environment with a recent version of Golang installed.

Use the ``go`` tool to download and install the ``cratedb-prometheus-adapter``
executable into your ``GOPATH``::

   go get github.com/crate/cratedb-prometheus-adapter


Set up Sandbox
==============

For working on the source code, it is advised to setup a development sandbox.
To start things off, clone the repository and change into the newly checked out
directory::

   mkdir -pv ${GOPATH}/src/github.com/crate
   cd ${GOPATH}/src/github.com/crate
   git clone https://github.com/crate/cratedb-prometheus-adapter.git
   cd cratedb-prometheus-adapter

To simply run the adapter, invoke::

   go run .

To print a default configuration to stdout, use::

   go run . -config.make

To build the ``cratedb-prometheus-adapter`` executable, run::

   go build

Unit tests
==========

To run the test suite, execute::

    go test -v

For running the linter, invoke::

    gofmt -l -d .

To format the files, according to what the linter says, run::

    gofmt -w .

Integration tests
=================

Run CrateDB and Prometheus::

    docker compose --file tests/docker-compose.yaml up

Set up Python sandbox::

    python3 -m venv .venv
    source .venv/bin/activate
    pip install -r requirements-test.txt

Run integration tests::

    pytest

Sandbox Deployment
==================

You can also use a Docker Compose service definition to run both CrateDB
and Prometheus on your workstation, for evaluation and testing purposes.

The setup will sample metrics each five seconds, for both ``prometheus`` and
``cratedb`` collection jobs::

    docker compose --file sandbox/docker-compose.yaml up

.. note::

    Please note that the Docker Compose configuration for the integration
    testing harness is not suitable for regular operations, because it uses
    a very high sampling rate. As such, it will significantly consume disk
    space when it is run for a longer time.

Provision database schema::

    crash < sql/ddl.sql

This currently needs to be accompanied by running the Prometheus Adapter
service on your workstation, like::

    go run .

Navigate to Prometheus UI, and verify that both endpoints are connected::

    open http://localhost:9090/targets

Set up Python sandbox::

    python3 -m venv .venv
    source .venv/bin/activate
    pip install -r requirements-test.txt

Submit Prometheus query expressions and display results::

    python sandbox/promquery.py

Background Services
-------------------

In order to start the services in the background, use DC's ``--detach``
option::

    docker compose --file sandbox/docker-compose.yaml up --detach

Then, for inspecting the log output of the background services, run::

    docker compose --file sandbox/docker-compose.yaml logs --follow

Maintaining dependencies
========================

Display available minor and patch upgrades for all direct and indirect dependencies::

   go list -u -m all

Update to the latest patch releases, including test dependencies::

   go get -u=patch -t
   go mod tidy


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

- Trigger the build/release job ``prometheus_adapter_release`` on
  `Jenkins CI`_ for the tag you just created.

Maintaining the jobs
====================

In order to create release archives, CI invokes::

    ./devtools/release.sh

As this is driven by Docker, it can be tested and maintained independently of CI.

.. _Jenkins CI: https://jenkins.crate.io


Building the Docker image
=========================

The project contains a ``Dockerfile`` which can be used to build a Docker
image::

   docker build --rm --tag crate/cratedb-prometheus-adapter .

When running the adapter inside Docker, you need to make sure that the running
container has access to the CrateDB instance(s) which it should write to / read
from.

To expose the ``/read``, ``/write`` and ``/metrics`` endpoints, the port
``9268`` must be published::

   docker run --rm -it \
   --publish=9268:9268 \
   crate/cratedb-prometheus-adapter

Since the default configuration would use ``localhost`` as CrateDB endpoint, a
``config.yml`` with the correct configuration needs to be mounted on
``/etc/cratedb-prometheus-adapter/config.yml``::

   docker run --rm -it \
   --publish=9268:9268 --volume=$(pwd)/config.yml:/etc/cratedb-prometheus-adapter/config.yaml \
   crate/cratedb-prometheus-adapter

