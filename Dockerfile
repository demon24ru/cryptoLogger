FROM golang:alpine3.15
WORKDIR /build
COPY . ./
RUN apk update && apk add --no-cache --virtual --update git gcc linux-pam-dev libc-dev
RUN go build ./cmd/cryptogalaxy -ldflags "-w -s" -buildvcs=false -o /build/cryptologger

FROM alpine:3.15
WORKDIR /cryptologger
RUN apk update && apk add --no-cache --virtual --update linux-pam-dev
COPY  --from=0 /build/cryptologger /cryptologger/cryptologger
COPY  config.json /cryptologger/config.json
ENTRYPOINT ["/cryptologger/cryptologger"]
