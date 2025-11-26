# syntax=docker/dockerfile:1

# ========= STAGE 1: Build =========
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go.mod + go.sum (you said go.sum exists)
COPY go.mod ./
RUN go mod download

# Copy source
COPY main.go ./
COPY static ./static
COPY templates ./templates
COPY nfs ./nfs

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /main main.go

# ========= STAGE 2: Runtime =========

# FROM scratch
FROM alpine:3.19
RUN apk add --no-cache curl iproute2 busybox-extras
# Debug stuff added

# Optional: for HTTPS calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /main /main

# Copy static assets and templates
COPY --from=builder /app/static /app/static
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/nfs /app/nfs


# Non-root user
USER 1001

# Working directory (matches ./static/)
WORKDIR /app

EXPOSE 8080

CMD ["/main"]
