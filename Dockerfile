# Сборка бинарника debuginfod
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /debuginfod ./cmd/debuginfod

FROM alpine:3.19
RUN apk add --no-cache ca-certificates sqlite-libs

COPY --from=builder /debuginfod /usr/local/bin/debuginfod

EXPOSE 8002
ENTRYPOINT ["/usr/local/bin/debuginfod"]
CMD ["-s", "/usr/lib/debug", "-p", "8002", "-d", "/data/debuginfod.sqlite"]
