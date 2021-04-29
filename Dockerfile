FROM golang:1.16 as builder
WORKDIR /go/src/github.com/crate/cratedb-prometheus-adapter
COPY . /go/src/github.com/crate/cratedb-prometheus-adapter/
RUN CGO_ENABLED=0 GOOS=linux go build

FROM scratch
EXPOSE 9268
COPY --from=builder /go/src/github.com/crate/cratedb-prometheus-adapter/cratedb-prometheus-adapter /usr/bin/cratedb-prometheus-adapter
WORKDIR /etc/cratedb-prometheus-adapter
COPY config.yml .
ENTRYPOINT ["/usr/bin/cratedb-prometheus-adapter"]
CMD ["-config.file=/etc/cratedb-prometheus-adapter/config.yml"]
