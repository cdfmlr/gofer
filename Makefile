PROG_NAME=gofer
PROG_VERSION := $(shell git rev-parse --short HEAD)

CGO_ENABLED=0
GOOS=linux
GOARCH=amd64

BINARY_NAME := $(PROG_NAME)-$(PROG_VERSION)-$(GOOS)_$(GOARCH)

.PHONY: all build archiveSrc clean

# all: archiveSrc build
all: build

build: $(BINARY_NAME)

$(BINARY_NAME):
	# build bin
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
    go build -ldflags "-s -w" -o $(BINARY_NAME) ./cmd

archiveSrc: $(PROG_NAME)-$(PROG_VERSION).tar.gz

$(PROG_NAME)-$(PROG_VERSION).tar.gz:
	# achieve src
	tar -czf $(PROG_NAME)-$(PROG_VERSION).tar.gz .

clean:
	rm -i $(PROG_NAME)-*

