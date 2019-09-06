FROM golang:1.12-alpine3.10

RUN apk update
RUN apk upgrade
RUN apk add bash ca-certificates git libc-dev expect make jq

ENV GO111MODULE=off
ENV PATH /go/bin:$PATH
ENV GOPATH /go
ENV DKGPATH /go/src/github.com/dgamingfoundation/dkglib
RUN mkdir -p /go/src/github.com/dgamingfoundation/dkglib

COPY . $DKGPATH

WORKDIR $DKGPATH

RUN go install $DKGPATH/

EXPOSE 26656
