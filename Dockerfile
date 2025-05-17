FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git bash zip

# Set working directory
WORKDIR /app


# Copy source code
COPY . .

RUN go mod tidy

# Build the application
RUN bash build.sh

# Use a smaller base image for the final image
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the binary from the builder stage
COPY --from=builder /app/build/aquatone_linux_amd64/aquatone /usr/local/bin/

# Set ownership and permissions
RUN chmod +x /usr/local/bin/aquatone && \
    chown appuser:appgroup /usr/local/bin/aquatone

# Switch to non-root user
USER appuser

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/aquatone"]

# Default command if none is provided
CMD ["--help"]

# Labels
LABEL org.opencontainers.image.source="https://github.com/mk990/aquatone"
LABEL org.opencontainers.image.description="A tool for visual inspection of websites across a large number of hosts"
LABEL org.opencontainers.image.licenses="MIT"
