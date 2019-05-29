FROM golang:1.12 AS build

COPY . /warscript-notify
WORKDIR /warscript-notify

RUN go build .

FROM alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
RUN update-ca-certificates 2>/dev/null || true

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2


COPY --from=build /warscript-notify/warscript-notify /warscript-notify

CMD [ "/warscript-notify" ]