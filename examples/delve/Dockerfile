FROM golang:1.22-bookworm

RUN apt-get update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		binutils \
		ca-certificates \
		curl \
		elfutils \
	&& rm -rf /var/lib/apt/lists/* \
	&& go install github.com/go-delve/delve/cmd/dlv@latest

ENV PATH="/go/bin:${PATH}"

WORKDIR /work
