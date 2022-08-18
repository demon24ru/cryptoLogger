FROM golang:alpine3.15
WORKDIR /build
COPY . ./
RUN apk update && apk add --no-cache --virtual --update git gcc linux-pam-dev libc-dev
RUN go build -ldflags "-w -s" -buildvcs=false -o /build/cryptogalaxy ./cmd/cryptogalaxy

FROM alpine:3.15
WORKDIR /cryptogalaxy
RUN apk update && apk add --no-cache --virtual --update linux-pam-dev
COPY  --from=0 /build/cryptogalaxy /cryptogalaxy/cryptogalaxy
#COPY  config.json /cryptogalaxy/config.json
ENTRYPOINT ["/cryptogalaxy/cryptogalaxy"]
