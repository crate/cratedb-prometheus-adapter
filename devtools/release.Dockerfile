# syntax=docker/dockerfile:1
#
# Run release archive builder within Docker container.
#

FROM golang:1.25-trixie

RUN apt-get update && apt-get --yes install zip

COPY . /src
WORKDIR /src

ARG NAME
ARG VERSION
RUN NAME=${NAME} VERSION=${VERSION} ./devtools/release_build.sh
