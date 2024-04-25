# syntax=docker/dockerfile:1
FROM alpine:3.19.1

ARG VERSION

LABEL org.opencontainers.image.ref.name="xelon-cloud-controller-manager" \
      org.opencontainers.image.source="https://github.com/Xelon-AG/xelon-cloud-controller-manager" \
      org.opencontainers.image.vendor="Xelon AG" \
      org.opencontainers.image.version="${VERSION:-local}"

RUN <<EOF
    set -ex
    apk add --no-cache ca-certificates
    # upgrade openssl package to fix vulnerabilities
    apk add --no-cache openssl
    rm -rf /var/cache/apk/*
EOF

COPY --chmod=755 xelon-cloud-controller-manager /bin/xelon-cloud-controller-manager

CMD ["/bin/xelon-cloud-controller-manager"]
