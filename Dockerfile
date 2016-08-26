# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/intervention-engine/integrator

WORKDIR /go/src/github.com/intervention-engine/integrator
RUN go get
RUN go build

# Install Dockerize to get support for waiting on another container's port to be available.
# This is needed here so docker-compose can be configured to wait on the ingest endpoint to be available.
RUN apt-get update && apt-get install -y wget
ENV DOCKERIZE_VERSION v0.2.0
RUN wget https://github.com/jwilder/dockerize/releases/download/$DOCKERIZE_VERSION/dockerize-linux-amd64-$DOCKERIZE_VERSION.tar.gz \
    && tar -C /usr/local/bin -xzvf dockerize-linux-amd64-$DOCKERIZE_VERSION.tar.gz
