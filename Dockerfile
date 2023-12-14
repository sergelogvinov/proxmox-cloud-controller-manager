# syntax = docker/dockerfile:1.5
########################################

FROM --platform=${BUILDPLATFORM} golang:1.21.5-alpine3.18 AS builder
RUN apk update && apk add --no-cache make
ENV GO111MODULE on
WORKDIR /src

COPY go.mod go.sum /src
RUN go mod download && go mod verify

COPY . .
ARG VERSION
ARG TAG
ARG SHA
RUN make build-all-archs

########################################

FROM --platform=${TARGETARCH} scratch AS release
LABEL org.opencontainers.image.source="https://github.com/sergelogvinov/proxmox-cloud-controller-manager" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.description="Proxmox VE CCM for Kubernetes"

COPY --from=gcr.io/distroless/static-debian12:nonroot . .
ARG TARGETARCH
COPY --from=builder /src/bin/proxmox-cloud-controller-manager-${TARGETARCH} /bin/proxmox-cloud-controller-manager

ENTRYPOINT ["/bin/proxmox-cloud-controller-manager"]
