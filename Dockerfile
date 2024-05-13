# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder

ENV CGO_ENABLED=0

# copy manifest files only to cache layer with dependencies
WORKDIR /src/app/
COPY go.mod go.sum /src/app/
RUN go mod download
# copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# build
RUN go build -o xelon-cloud-controller-manager -ldflags="-s -w" -trimpath cmd/xelon-cloud-controller-manager/main.go



FROM gcr.io/distroless/static:nonroot AS production

ARG VERSION

LABEL org.opencontainers.image.ref.name="xelon-cloud-controller-manager" \
      org.opencontainers.image.source="https://github.com/Xelon-AG/xelon-cloud-controller-manager" \
      org.opencontainers.image.vendor="Xelon AG" \
      org.opencontainers.image.version="${VERSION:-local}"

WORKDIR /
USER 65532:65532

COPY --from=builder --chmod=755 /src/app/xelon-cloud-controller-manager /xelon-cloud-controller-manager

ENTRYPOINT ["/xelon-cloud-controller-manager"]
