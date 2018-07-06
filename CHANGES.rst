======================================
CHANGES for Prometheus CrateDB Adapter
======================================

Unreleased
==========

- Use Postgres wire protocol (pgx client library) to connect to CrateDB:

  - This change requires CrateDB 3.1.0 or newer!

  - Connections can be configured via ``crate.yml`` configuration file using
    the ``-config.file`` flag.

  - Added support for multiple endpoints.

2017-06-11 0.1
==============

- Unofficial experimental release
