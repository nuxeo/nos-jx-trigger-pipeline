FROM golang:1.12

WORKDIR /go/src/github.com/jstrachan/trigger-pipeline

COPY . /go/src/github.com/jstrachan/trigger-pipeline

RUN make linux

FROM centos:7

RUN yum install -y git

ENTRYPOINT ["tp", "trigger"]

COPY --from=0 /go/src/github.com/jstrachan/trigger-pipeline/build/linux/tp /usr/local/bin
