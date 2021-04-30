#!/bin/bash
#
# Build release archives for Linux, Darwin and Windows.
#

# Compute package base name
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


# Build program for multiple platforms

export GOOS=linux
export GOARCH=386
FILENAME="${BASE_FILENAME}.${GOOS}.${GOARCH}.tar.gz"
echo "Building ${FILENAME}"
go build
tar -cvzf ${FILENAME} ${NAME}
rm ${NAME}

export GOOS=linux
export GOARCH=amd64
FILENAME="${BASE_FILENAME}.${GOOS}.${GOARCH}.tar.gz"
echo "Building ${FILENAME}"
go build
tar -cvzf ${FILENAME} ${NAME}
rm ${NAME}

export GOOS=darwin
export GOARCH=amd64
FILENAME="${BASE_FILENAME}.${GOOS}.${GOARCH}.tar.gz"
echo "Building ${FILENAME}"
go build
tar -cvzf ${FILENAME} ${NAME}
rm ${NAME}

export GOOS=windows
export GOARCH=amd64
FILENAME="${BASE_FILENAME}.${GOOS}.${GOARCH}.zip"
echo "Building ${FILENAME}"
go build -o ${NAME}.exe
zip ${FILENAME} ${NAME}.exe
rm ${NAME}.exe
