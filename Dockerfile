# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder

ARG GIT_COMMIT
ARG GIT_TREE_STATE
ARG SOURCE_DATE_EPOCH
ARG VERSION

ENV CGO_ENABLED=0

# copy manifest files only to cache layer with dependencies
WORKDIR /src/app/
COPY go.mod go.sum /src/app/
RUN go mod download
# copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# build
RUN go build -trimpath \
    -ldflags="-s -w \
    -X github.com/Xelon-AG/xelon-cloud-controller-manager/internal/xelon.gitCommit=${GIT_COMMIT:-none} \
    -X github.com/Xelon-AG/xelon-cloud-controller-manager/internal/xelon.gitTreeState=${GIT_TREE_STATE:-none} \
    -X github.com/Xelon-AG/xelon-cloud-controller-manager/internal/xelon.sourceDateEpoch=${SOURCE_DATE_EPOCH:-0} \
    -X github.com/Xelon-AG/xelon-cloud-controller-manager/internal/xelon.version=${VERSION:-local}" \
    -o xelon-cloud-controller-manager cmd/xelon-cloud-controller-manager/main.go



FROM gcr.io/distroless/static:nonroot AS production

ARG VERSION

LABEL org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.ref.name="xelon-cloud-controller-manager" \
      org.opencontainers.image.source="https://github.com/Xelon-AG/xelon-cloud-controller-manager" \
      org.opencontainers.image.vendor="Xelon AG" \
      org.opencontainers.image.version="${VERSION:-local}"

WORKDIR /
USER 65532:65532

COPY --from=builder --chmod=755 /src/app/xelon-cloud-controller-manager /xelon-cloud-controller-manager

ENTRYPOINT ["/xelon-cloud-controller-manager"]
