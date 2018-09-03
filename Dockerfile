FROM golang:alpine

ENV BK_ORGANIZATION=""
ENV BK_ACCESS_TOKEN=""

RUN mkdir -p /go/src/github.com/nutthaka/buildkite_exporter

WORKDIR /go/src/github.com/nutthaka/buildkite_exporter
ADD . .

RUN go build -o /go/bin/buildkite_exporter 
EXPOSE 9260
CMD /go/bin/buildkite_exporter -buildkite.organization ${BK_ORGANIZATION} -buildkite.token ${BK_ACCESS_TOKEN}