FROM golang:1.22-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY main.go ./
COPY internal/ ./internal/

ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /out/shadowschema main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlite3-0 \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/shadowschema /app/shadowschema

RUN mkdir -p /app/certs /app/data

VOLUME ["/app/certs", "/app/data"]

EXPOSE 38080 38081

ENTRYPOINT ["/app/shadowschema"]