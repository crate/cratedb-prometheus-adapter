# CrateDB Prometheus Adapter

This is an adapter that accepts Prometheus remote read/write requests,
and sends them on to CrateDB. This allows using CrateDB as long term storage
for Prometheus.

Requires CrateDB 2.2.

## Building

```
go get github.com/crate/crate_adapter
cd ${GOPATH-$HOME/go}/src/github.com/crate/crate_adapter
go build
```

## Usage

Create the following table in your CrateDB database:

```
CREATE TABLE "metrics" (
  "timestamp" TIMESTAMP,
  "labels_hash" STRING,
  "labels" OBJECT(DYNAMIC),
  "value" DOUBLE,
  "valueRaw" LONG,
  PRIMARY KEY ("timestamp", "labels_hash")
);
```

Then run the adapter:

```
./crate_adapter
```

By default the adapter will listen on port 9268, and talk to the local CrateDB running on port 4200.
This is configurable via command line flags, which you can see by passing the `-h` flag.


## Prometheus Configuration

Add the following to your `prometheus.yml`:

```
remote_write:
   - url: http://localhost:9268/write
remote_read:
   - url: http://localhost:9268/read
```

The adapter also exposes Prometheus metrics on /metrics, and can be scraped in the usual fashion.
