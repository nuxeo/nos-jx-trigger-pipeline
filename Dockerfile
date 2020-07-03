FROM centos:7

RUN yum install -y git

ENTRYPOINT ["tp", "trigger"]

COPY build/linux/tp /usr/local/bin
