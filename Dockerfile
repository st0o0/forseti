FROM golang:1.26.6-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN [ -f go.sum ] && go mod download || go mod tidy
COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /forseti ./cmd/forseti

FROM scratch

COPY --from=build /forseti /forseti
COPY LICENSE.md /LICENSE.md

EXPOSE 9099

HEALTHCHECK --interval=30s --timeout=10s \
           --start-period=15s --retries=3 \
  CMD ["/forseti", "healthcheck"]

ENTRYPOINT ["/forseti"]
