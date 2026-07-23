# Multi-stage build for the ZKTeco gateway.
#
# --platform=$BUILDPLATFORM: compile natively on the build host and cross-compile
# to the target arch. Pure Go (no cgo) → no C toolchain needed. This is what lets
# the same Dockerfile produce an arm64 image for the Raspberry Pi from an amd64
# CI runner (build with: docker buildx build --platform linux/arm64 ...).
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app

COPY src/ .

# -mod=mod lets the first build resolve/download modules and fill go.sum even if
# one was not committed (run `go mod tidy` locally to pin it). Needs internet on
# the build host, which a Pi / CI runner has.
ENV CGO_ENABLED=0 GOFLAGS=-mod=mod
RUN go mod download

ARG TARGETOS
ARG TARGETARCH
# TARGETOS/TARGETARCH come from buildx per target platform, so the binary's arch
# always matches the runtime base image. Do NOT hardcode GOARCH.
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm64} go build -trimpath -ldflags="-s -w" -o /out/zkbio .

FROM alpine:latest
# ca-certificates: outbound HTTPS to the HR service.
# tzdata: the gateway interprets naive device timestamps in a configured TZ.
# wget (busybox) is already present for the container healthcheck.
RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /out/zkbio /usr/bin/zkbio

# Management API + ADMS/iclock push endpoints listen here.
EXPOSE 8080

ENTRYPOINT ["/usr/bin/zkbio"]
