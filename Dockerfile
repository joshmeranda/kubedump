FROM alpine:3.16.0

RUN apk add libc6-compat

COPY bin/kubedump-server /bin

ENTRYPOINT kubedump-server