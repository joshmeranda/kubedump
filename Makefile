SOURCES=
TEST_PATHS=./pkg

KDUMP_VERSION=$(shell tools/version.bash get)
IMAGE_TAG=joshmeranda/kdump:${KDUMP_VERSION}

# # # # # # # # # # # # # # # # # # # #
# Go commands                         #
# # # # # # # # # # # # # # # # # # # #
GO_BUILD=go build
GO_FMT=go fmt -x
GO_TEST=go test -test.parallel 1

ifdef VERBOSE
	Go_BUILD += -v
	GO_FMT += -x
	GO_TEST += -test.v

	RM += --verbose
endif

# # # # # # # # # # # # # # # # # # # #
# Help text for easier Makefile usage #
# # # # # # # # # # # # # # # # # # # #
.PHONY: help

help:
	@echo "Usage: make [TARGETS]... [VALUES]"
	@echo ""
	@echo "Targets:"
	@echo "  kdump           build the kdump binary"
	@echo "  kdump-server    build the kdump server binary"
	@echo "  docker          builder the kdump-serve image"
	@echo "  all             build all binaries and docker images"
	@echo "  test            run all tests"
	@echo "  mostly-clean    clean any project generated files (not-including deliverables)"
	@echo "  clean           clean built and generated files"
	@echo "  fmt             run the source through the builtin go formatter"
	@echo ""
	@echo "Values:"
	@echo "  VERBOSE         if set various recipes are run with verbose output"

# # # # # # # # # # # # # # # # # # # #
# Source and binary build / compile   #
# # # # # # # # # # # # # # # # # # # #
.PHONY: kdump kdump-server

all: kdump kdump-server

kdump: bin/kdump

bin/kdump: ${SOURCES} cmd/kdump/main.go
	${GO_BUILD} -o $@ ./cmd/kdump

kdump-server: bin/kdump-server

bin/kdump-server: ${SOURCES} cmd/kdump-server/main.go
	${GO_BUILD} -o $@ ./cmd/kdump-server

# # # # # # # # # # # # # # # # # # # #
# BUild docker images                 #
# # # # # # # # # # # # # # # # # # # #
.PHONY: docker

docker: kdump-server
	sudo docker build --tag ${IMAGE_TAG} .

# # # # # # # # # # # # # # # # # # # #
# Run go tests                        #
# # # # # # # # # # # # # # # # # # # #
test: ${SOURCES}
	${GO_TEST} ${TEST_PATHS}

# # # # # # # # # # # # # # # # # # # #
# Project management recipes          #
# # # # # # # # # # # # # # # # # # # #

.PHONY: clean fmt mostly-clean

mostly-clean:

clean: mostly-clean
	${RM} --recursive bin

fmt:
	${GO_FMT} ./cmd/kdump
