FROM golang:1.13.4-alpine as builder

RUN apk add --no-cache gcc g++ bash git

WORKDIR $GOPATH/src/github.com/toni-moreno/influxdb-srelay

COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY .git .git
COPY build.go ./

RUN go run build.go  build

FROM alpine:latest

COPY --from=builder /go/src/github.com/toni-moreno/influxdb-srelay/bin/influxdb-srelay ./bin/
COPY ./examples/*.conf /etc/influxdb-srelay/
COPY ./examples/sample.influxdb-srelay.conf /etc/influxdb-srelay/influxdb-srelay.conf
RUN mkdir -p /var/log/influxdb-srelay

ENTRYPOINT [ "/bin/influxdb-srelay" ]

CMD [ "-config", "/etc/influxdb-srelay/influxdb-srelay.conf" , "-logs","/var/log/influxdb-srelay" ]
# EOF
