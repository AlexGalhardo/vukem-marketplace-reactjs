# Recorder for docs/media/tests.gif: VHS plus the Go toolchain the tests need.
FROM golang:1.27.3-bookworm AS go
FROM ghcr.io/charmbracelet/vhs:v0.11.0
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:$PATH GOFLAGS=-buildvcs=false GOTOOLCHAIN=local
