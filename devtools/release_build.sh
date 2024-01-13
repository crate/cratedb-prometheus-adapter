#!/bin/bash
#
# Build release archives for Linux, Darwin, and Windows.
#

# Compute package names.
if [[ -z "${NAME}" ]]; then
  echo "ERROR: Unable to determine package name"
  exit 1
fi

if [[ ! -z "${VERSION}" ]]; then
  SUFFIX=${VERSION}
else
  COMMIT="$(git rev-parse --short HEAD)"
  SUFFIX="$(date +%Y%m%d%H)-${COMMIT}"
fi

if [[ -z "${SUFFIX}" ]]; then
  echo "ERROR: Unable to determine version"
  exit 1
fi

BASE_FILENAME="${NAME}-${SUFFIX}"


# Utility functions.

function build_tarball() {
  os=$1
  arch=$2
  filename="${BASE_FILENAME}.${os}.${arch}.tar.gz"
  echo "Building ${filename}"
  go build
  tar -cvzf ${filename} ${NAME}
  rm ${NAME}
}

function build_windows_zipball() {
  os=$1
  arch=$2
  filename="${BASE_FILENAME}.${os}.${arch}.zip"
  echo "Building ${filename}"
  go build -o ${NAME}.exe
  zip ${filename} ${NAME}.exe
  rm ${NAME}.exe
}


# Build program for multiple platforms.
function main() {
  build_tarball linux amd64
  build_tarball linux arm64
  build_tarball linux 386
  build_tarball linux arm
  build_tarball darwin amd64
  build_tarball darwin arm64
  build_windows_zipball windows amd64
  build_windows_zipball windows arm64
}

main
