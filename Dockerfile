# Сборка debuginfod-go (Debian bookworm — совместимо с Astra/Ubuntu, без Alpine apk).
FROM golang:1.21-bookworm AS builder

RUN apt-get update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		gcc \
		libc6-dev \
		libsqlite3-dev \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /debuginfod ./cmd/debuginfod

FROM debian:bookworm-slim

RUN apt-get update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		ca-certificates \
		curl \
		libsqlite3-0 \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /data

COPY --from=builder /debuginfod /usr/local/bin/debuginfod

EXPOSE 8002
ENTRYPOINT ["/usr/local/bin/debuginfod"]
CMD ["-p", "8002"]
