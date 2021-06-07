FROM golang:1.15 as reporterBuilder
WORKDIR /root/
COPY . /root/
ENV CGO_ENABLED=0
RUN go build -o /usr/local/bin/e2e-reporter /root/tests/e2e/tools/runner/reporter/*.go

FROM golang:1.15 as e2eBuilder
WORKDIR /root/
COPY . /root/
ENV CGO_ENABLED=0
RUN go test -c -o /usr/local/bin/e2e.test /root/tests/e2e/cases/*.go

FROM alpine:3.15

COPY --from=reporterBuilder /usr/local/bin/e2e-reporter /usr/local/bin/e2e-reporter
COPY --from=e2eBuilder /usr/local/bin/e2e.test /usr/local/bin/e2e.test

USER root
ENTRYPOINT ["/usr/local/bin/e2e-reporter"]
