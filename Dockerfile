FROM golang:1.13.4-alpine as builder

RUN apk add --no-cache gcc g++ bash git

WORKDIR $GOPATH/src/github.com/toni-moreno/influxdb-srelay

COPY go.mod go.sum ./
COPY backend/ ./backend/
COPY cluster/ ./cluster/
COPY config/ ./config/
COPY prometheus/ ./prometheus/
COPY relay/ ./relay/
COPY relayctx/ ./relayctx/
COPY relayservice/ ./relayservice/
COPY utils/ ./utils/
COPY main.go ./
COPY .git .git
COPY build.go ./

#COPY . ./
RUN ls -lisa ./
RUN go run build.go  build

FROM alpine:latest

COPY --from=builder /go/src/github.com/toni-moreno/influxdb-srelay/bin/influxdb-srelay ./bin/
COPY ./examples/*.conf /etc/influxdb-srelay/
COPY ./examples/sample.influxdb-srelay.conf /etc/influxdb-srelay/influxdb-srelay.conf
RUN mkdir -p /var/log/influxdb-srelay

ENTRYPOINT [ "/bin/influxdb-srelay" ]

CMD [ "-config", "/etc/influxdb-srelay/influxdb-srelay.conf" , "-logs","/var/log/influxdb-srelay" ]
# EOF
