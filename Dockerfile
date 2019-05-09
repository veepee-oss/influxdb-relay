# The MIT License (MIT)
#
# Copyright (c) 2018 Vente-Privée
#
# Permission is hereby granted, free of  charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction,  including without limitation the rights
# to use,  copy, modify,  merge, publish,  distribute, sublicense,  and/or sell
# copies  of the  Software,  and to  permit  persons to  whom  the Software  is
# furnished to do so, subject to the following conditions:
#
# The above  copyright notice and this  permission notice shall be  included in
# all copies or substantial portions of the Software.
#
# THE SOFTWARE  IS PROVIDED "AS IS",  WITHOUT WARRANTY OF ANY  KIND, EXPRESS OR
# IMPLIED,  INCLUDING BUT  NOT LIMITED  TO THE  WARRANTIES OF  MERCHANTABILITY,
# FITNESS FOR A  PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO  EVENT SHALL THE
# AUTHORS  OR COPYRIGHT  HOLDERS  BE LIABLE  FOR ANY  CLAIM,  DAMAGES OR  OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

FROM golang:alpine AS build-env

# Add missing git
RUN apk update && \
    apk upgrade && \
    apk add git

# Install
RUN go get -u github.com/toni-moreno/influxdb-srelay && \
    mv /go/bin/influxdb-srelay /usr/bin/influxdb-srelay && \
    chmod 755 /usr/bin/influxdb-srelay && \
    mkdir /etc/influxdb-srelay && \
    touch /etc/influxdb-srelay/influxdb-srelay.conf

FROM alpine:latest

COPY --from=build-env /usr/bin/influxdb-srelay /
COPY --from=build-env /go/src/github.com/toni-moreno/influxdb-srelay/example/sample.influxdb-srelay.conf /etc/influxdb-srelay/
RUN mkdir -p /var/log/influxdb-srelay

ENTRYPOINT [ "/usr/bin/influxdb-srelay" ]

CMD [ "-config", "/etc/influxdb-srelay/influxdb-srelay.conf" , "-logdir","/var/log/influxdb-srelay" ]
# EOF
