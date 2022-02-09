# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/


COPY hack/boilerplate.go.txt hack/boilerplate.go.txt
COPY scripts/csv-generator.go scripts/csv-generator.go

# Copy the go source
COPY Makefile Makefile
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on make manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on make csv-generator


FROM registry.access.redhat.com/ubi8/ubi-minimal
LABEL org.kubevirt.hco.csv-generator.v1="/csv-generator"

WORKDIR /

COPY --from=builder /workspace/bin/manager .
COPY data/ data/

# Copy csv generator
COPY --from=builder /workspace/bin/csv-generator .

ENTRYPOINT ["/manager"]
