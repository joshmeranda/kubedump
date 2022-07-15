FROM golang:1.18 as builder

COPY bin/kubedump-server /usr/bin

ENTRYPOINT kubedump-server