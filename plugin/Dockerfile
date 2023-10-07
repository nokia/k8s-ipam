# syntax=docker/dockerfile:1.2
FROM golang:1.20.4-alpine3.18 as builder
WORKDIR /src
ENV CGO_ENABLED 0
COPY go.* /src/
RUN go mod download
COPY build/ build/
COPY pkg/ pkg/
COPY cmd/ cmd/

#---
FROM builder as binary
ARG VERSION
RUN export DATE=$(date '+%Y-%m-%d %H:%M') && CGO_ENABLED=0 go build -o k8s-ipam-cni \
  -ldflags="-s -X 'github.com/k8s-ipam/plugin/build.Version=${VERSION}' \
  -X 'github.com/k8s-ipam/plugin/build.Date=${DATE}'" \
  cmd/plugin/main.go

FROM scratch as bin
COPY --from=binary /src/k8s-ipam-cni /

#---
FROM builder as test-run
USER root
RUN CGO_ENABLED=0 go test

#---
FROM scratch AS test 
COPY --from=test-run /src/gounit.txt /




