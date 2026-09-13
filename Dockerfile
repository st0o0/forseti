# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
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
