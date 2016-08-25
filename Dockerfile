# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/intervention-engine/integrator

WORKDIR /go/src/github.com/intervention-engine/integrator
RUN go get
RUN go build
