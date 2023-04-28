# syntax = docker/dockerfile:1.4
########################################

FROM --platform=${BUILDPLATFORM} golang:1.20.3-alpine3.17 AS builder
RUN apk update && apk add --no-cache make
ENV GO111MODULE on
WORKDIR /src

COPY go.mod go.sum /src
RUN go mod download && go mod verify

COPY . .
ARG TAG
RUN make build-all-archs

########################################

FROM --platform=${TARGETARCH} gcr.io/distroless/static-debian11:nonroot AS release
LABEL org.opencontainers.image.source https://github.com/sergelogvinov/proxmox-cloud-controller-manager

ARG TARGETARCH
COPY --from=builder /src/bin/proxmox-cloud-controller-manager-${TARGETARCH} /proxmox-cloud-controller-manager

ENTRYPOINT ["/proxmox-cloud-controller-manager"]
