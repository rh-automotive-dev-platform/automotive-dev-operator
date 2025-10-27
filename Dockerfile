# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.24.6 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/main.go cmd/main.go
COPY cmd/build-api/main.go cmd/build-api/main.go
COPY cmd/init-secrets/main.go cmd/init-secrets/main.go
COPY api/ api/
COPY internal/ internal/

ENV CGO_ENABLED=0
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w" -o manager cmd/main.go
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w" -o build-api cmd/build-api/main.go
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w" -o init-secrets cmd/init-secrets/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/build-api .
COPY --from=builder /workspace/init-secrets .
USER 65532:65532

ENTRYPOINT ["/manager"]
