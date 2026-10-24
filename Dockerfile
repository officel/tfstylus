# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build

WORKDIR /build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -mod=readonly -trimpath -buildvcs=false \
    -ldflags="-s -w" -o /out/tfstylus ./cmd/tfstylus

FROM scratch

ENV PATH=/usr/local/bin HOME=/home/nonroot
COPY --from=build --chmod=0555 /out/tfstylus /usr/local/bin/tfstylus
COPY --chmod=0444 LICENSE /licenses/LICENSE
USER 65532:65532
WORKDIR /src

ENTRYPOINT ["/usr/local/bin/tfstylus"]
CMD ["plan"]
