FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev libstdc++-dev

# Copy go.mod and go.sum first to leverage layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -tags goolm -o /usr/bin/mautrix-mattermost ./main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /data

# Install runtime dependencies (ca-certificates for HTTPS)
RUN apk add --no-cache ca-certificates su-exec

COPY --from=builder /usr/bin/mautrix-mattermost /usr/bin/mautrix-mattermost
# Copy helper script if needed

CMD ["/usr/bin/mautrix-mattermost"]
