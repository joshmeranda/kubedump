FROM golang:1.18 as builder

COPY bin/kdump-server /usr/bin