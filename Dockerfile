# GitCode CLI Docker Image
# Uses pre-built binary from GoReleaser

# 3.19 reached end of life on 2025-11-01. 3.22 is supported until 2027-05-01.
# TODO: pin the multi-arch index digest once registry access is available
# (docker manifest inspect alpine:3.22) — a per-arch child digest would
# break the linux/arm64 build.
FROM alpine:3.22

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    bash \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 gc && \
    adduser -u 1000 -G gc -s /bin/sh -D gc

WORKDIR /home/gc

# Copy pre-built binary (provided by GoReleaser)
COPY gc /usr/local/bin/gc

# Copy completions
COPY completions /usr/share/completions

# Set ownership
RUN chown -R gc:gc /home/gc && \
    chmod +x /usr/local/bin/gc

# Switch to non-root user
USER gc

# Set environment
ENV PATH="/usr/local/bin:${PATH}"
ENV GC_PAGER=less

# Default command
ENTRYPOINT ["gc"]
CMD ["--help"]

# Labels
LABEL org.opencontainers.image.title="GitCode CLI"
LABEL org.opencontainers.image.description="Command line tool for GitCode"
LABEL org.opencontainers.image.url="https://gitcode.com"
LABEL org.opencontainers.image.source="https://github.com/atomgit-cli/cli"
LABEL org.opencontainers.image.vendor="GitCode"
