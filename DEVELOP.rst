===============
Developer Guide
===============

Setup
=====

To start things off, clone the repository::

    $ git clone https://github.com/crate/crate_adapter
    $ cd crate_adapter

Then, run::

    $ go build

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

On master:

- Update the release notes to reflect the release

- Update the ``version`` constant to the next minor version and suffix it with
  ``-dev`` (e.g. ``1.3.0-dev``)

Next:

- Trigger the build/release script on `Jenkins CI`_ for the newly created tag

.. _Jenkins CI: https://jenkins.crate.io
