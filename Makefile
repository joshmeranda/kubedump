SOURCES=./pkg/*.go ./pkg/collector/*.go ./pkg/filter/*.go
TEST_PATHS=./pkg ./pkg/filter/

KUBEDUMP_VERSION=$(shell tools/version.bash get)
IMAGE_TAG=joshmeranda/kubedump-server:${KUBEDUMP_VERSION}

CHARTS_DIR=charts

BUILDER=docker

HELM_PACKAGE=helm package

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

	HELM_PACKAGE += --debug

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
	@echo "  kubedump           build the kubedump binary"
	@echo "  kubedump-server    build the kubedump server binary"
	@echo "  docker          builder the kubedump-serve image"
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
.PHONY: kubedump kubedump-server

all: docker charts kubedump

kubedump: bin/kubedump go.mod

bin/kubedump: go.mod ${SOURCES} cmd/kubedump/*.go
	${GO_BUILD} -o $@ ./cmd/kubedump

kubedump-server: bin/kubedump-server

bin/kubedump-server: go.mod ${SOURCES} cmd/kubedump-server/*.go
	${GO_BUILD} -o $@ ./cmd/kubedump-server

# # # # # # # # # # # # # # # # # # # #
# Build docker images                 #
# # # # # # # # # # # # # # # # # # # #
.PHONY: docker

docker: kubedump-server
	${BUILDER} build --tag ${IMAGE_TAG} .

# # # # # # # # # # # # # # # # # # # #
# Package kubedump-seve helm chart    #
# # # # # # # # # # # # # # # # # # # #
.PHONY: charts

charts:
	for chart in $$(ls "${CHARTS_DIR}"); do \
	   ${HELM_PACKAGE} --destination artifacts "${CHARTS_DIR}/$${chart}";						\
	done

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
	${RM} --recursive artifacts bin

fmt:
	${GO_FMT} ./cmd/kubedump
