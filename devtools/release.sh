#!/bin/bash
#
# Create release archives for https://cdn.crate.io/downloads/dist/prometheus/.
#

# Ensure build folder exists and prune it.
mkdir -p build
rm build/*

# Designate package name.
NAME="cratedb-prometheus-adapter"

# Use most recent git tag as version number.
# TODO: Should this be made more elaborate, to also account for patch and nightly releases?
VERSION="$(git tag --sort version:refname | tail -n1)"

# Build program for multiple architectures.
docker build --file=devtools/release.Dockerfile --tag=build-cpa-all --build-arg="NAME=${NAME}" --build-arg="VERSION=${VERSION}" --progress=plain .

# Extract build artefacts from Docker image.
docker run --volume=$PWD/build:/build --rm build-cpa-all sh -c "cp /src/*.tar.gz /src/*.zip /build/"
