# syntax=docker/dockerfile:1@sha256:572cebbc2b15e6f84c077742952674889cb63cd5e9b80abdffcbe587b6f19183

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
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
COPY LICENSE /

USER 65534:65534

EXPOSE 9099

HEALTHCHECK --interval=30s --timeout=10s \
           --start-period=30s --retries=3 \
  CMD ["/forseti", "healthcheck", "--config", "/config/forseti.yml"]

ENTRYPOINT ["/forseti"]
