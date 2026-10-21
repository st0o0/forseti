# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
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
