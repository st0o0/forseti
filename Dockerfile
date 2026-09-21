# syntax=docker/dockerfile:1@sha256:ecfaec9ed6d810b56388c508f4121597bfbba70d41a6dfeee4d8cad5f295fc32

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /forseti ./cmd/forseti

FROM scratch
COPY --from=build /forseti /forseti
COPY LICENSE.md /LICENSE.md

EXPOSE 9099

HEALTHCHECK --interval=30s --timeout=10s \
           --start-period=30s --retries=3 \
  CMD ["/forseti", "healthcheck", "--config", "/config/forseti.yml"]

ENTRYPOINT ["/forseti"]
