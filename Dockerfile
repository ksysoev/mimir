FROM golang:1.26.1-alpine AS builder

ARG VERSION=${VERSION}

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o mimir -ldflags "-X main.version=$VERSION -X main.name=mimir" ./cmd/mimir/main.go

FROM scratch

COPY --from=builder /app/mimir .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD ["/mimir", "health"]

EXPOSE 7000

ENTRYPOINT ["/mimir"]
# Empty --config disables config file loading; the container is configured
# entirely via environment variables.
CMD ["serve", "--config", ""]
