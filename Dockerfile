FROM golang:1.9.2 as builder
WORKDIR /go/src/github.com/crate/crate_adapter
COPY . /go/src/github.com/crate/crate_adapter/
RUN CGO_ENABLED=0 GOOS=linux go build

FROM alpine:3.7
EXPOSE 9268
WORKDIR /root/
COPY --from=builder /go/src/github.com/crate/crate_adapter/crate_adapter .
ENTRYPOINT ["./crate_adapter"]
