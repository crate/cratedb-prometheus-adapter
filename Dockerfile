FROM golang:1.16 as builder
WORKDIR /go/src/github.com/crate/crate_adapter
COPY . /go/src/github.com/crate/crate_adapter/
RUN CGO_ENABLED=0 GOOS=linux go build

FROM alpine:3.13
EXPOSE 9268
COPY --from=builder /go/src/github.com/crate/crate_adapter/crate_adapter /usr/bin/crate_adapter
WORKDIR /etc/crate_adapter
COPY config.yml .
ENTRYPOINT ["/usr/bin/crate_adapter"]
CMD ["-config.file=/etc/crate_adapter/config.yml"]
