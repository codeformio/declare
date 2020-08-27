# Build the manager binary
FROM golang:1.14-alpine as builder


WORKDIR /workspace


# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY vendor/ vendor/
COPY template/ template/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod vendor -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.12
WORKDIR /bin

RUN apk add curl && curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl && apk del curl
RUN chmod +x ./kubectl

COPY --from=builder /workspace/manager /bin
#USER nonroot:nonroot

ENTRYPOINT ["/bin/manager"]
