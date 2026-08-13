# Single-container build of the Go binary, per docs/STACK.md's deploy target ("one Docker
# container ... matches the run-locally-now, self-host-on-a-VPS-later trajectory").
#
# contract.schema.json and the SQLite migrations are go:embed'd into the binary at build time, so
# only the compiled server + the adapters directory (subprocess binaries + manifest.json) need to
# ship in the final image.

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/adapters/stub/stub ./adapters/stub
RUN CGO_ENABLED=0 go build -o /out/adapters/host-metrics-ssh/host-metrics-ssh ./adapters/host-metrics-ssh
RUN CGO_ENABLED=0 go build -o /out/adapters/uptime-http/uptime-http ./adapters/uptime-http

FROM alpine:3.20
RUN addgroup -S sre-kit && adduser -S -G sre-kit sre-kit
WORKDIR /app

COPY --from=build /out/server ./server
COPY --from=build /out/adapters/stub/stub ./adapters/stub/stub
COPY adapters/stub/manifest.json ./adapters/stub/manifest.json
COPY --from=build /out/adapters/host-metrics-ssh/host-metrics-ssh ./adapters/host-metrics-ssh/host-metrics-ssh
COPY adapters/host-metrics-ssh/manifest.json ./adapters/host-metrics-ssh/manifest.json
COPY --from=build /out/adapters/uptime-http/uptime-http ./adapters/uptime-http/uptime-http
COPY adapters/uptime-http/manifest.json ./adapters/uptime-http/manifest.json

RUN mkdir -p /app/data && chown -R sre-kit:sre-kit /app
USER sre-kit

ENV SRE_KIT_ADDR=:8080
ENV SRE_KIT_DB_PATH=/app/data/sre-kit.db
ENV SRE_KIT_SECRETS_PATH=/app/data/secrets.enc.json
ENV SRE_KIT_ADAPTERS_DIR=/app/adapters

EXPOSE 8080
ENTRYPOINT ["./server"]
