FROM golang:1.22-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY main.go ./
COPY internal/ ./internal/

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/shadowschema main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    default-jre-headless \
    gnupg \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/shadowschema /app/shadowschema

RUN mkdir -p /app/certs

VOLUME ["/app/certs"]

EXPOSE 38080 38081

ENTRYPOINT ["/app/shadowschema"]