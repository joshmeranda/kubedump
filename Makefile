SOURCES=./pkg/cmd/*.go ./pkg/cmd/kubedump-server/*.go ./pkg/cmd/kubedump/*.go ./pkg/*.go ./pkg/filter/*.go ./pkg/controller/*.go

UNIT_TEST_PATHS=./pkg/filter ./pkg/controller ./pkg/http ./pkg/cmd
INTEGRATION_TEST_PATHS=./tests
TEST_PATHS=${UNIT_TEST_PATHS} ${INTEGRATION_TEST_PATHS}

KUBEDUMP_VERSION=$(shell tools/version.bash get)
IMAGE_TAG=joshmeranda/kubedump-server:${KUBEDUMP_VERSION}

CHARTS_DIR=charts

BUILDER=docker

HELM_PACKAGE=helm package

KUBEDUMP_VERSION=$(shell tools/version.bash get)

# # # # # # # # # # # # # # # # # # # #
# Go commands                         #
# # # # # # # # # # # # # # # # # # # #
GO_BUILD=go build -ldflags "-X kubedump/pkg/cmd.Version=${KUBEDUMP_VERSION}"
GO_FMT=go fmt
GO_TEST=go test

ifdef VERBOSE
	GO_BUILD += -v
	GO_FMT += -x
	GO_TEST += -test.v

	HELM_PACKAGE += --debug

	RM += --verbose
endif

# # # # # # # # # # # # # # # # # # # #
# Help text for easier Makefile usage #
# # # # # # # # # # # # # # # # # # # #
.PHONY: help sandbozx

help:
	@echo "Usage: make [TARGETS]... [VALUES]"
	@echo ""
	@echo "Targets:"
	@echo "  kubedump           build the kubedump binary"
	@echo "  kubedump-server    build the kubedump server binary"
	@echo "  generate            run code generation"
	@echo "  docker             builder the kubedump-serve image"
	@echo "  all                build all binaries and docker images"
	@echo "  test               run all tests"
	@echo "  mostly-clean       clean any project generated files (not-including deliverables)"
	@echo "  clean              clean built and generated files"
	@echo "  fmt                run the source through the builtin go formatter"
	@echo ""
	@echo "Values:"
	@echo "  VERBOSE            if set various recipes are run with verbose output"

# # # # # # # # # # # # # # # # # # # #
# code generation                     #
# # # # # # # # # # # # # # # # # # # #
HANDLER_TEMPLATE=pkg/codegen/handler.tpl

YYPARSER=pkg/filter/yyparser.go
YACC_FILE=pkg/codegen/parser.y

.PHONY: generate

${YYPARSER}: ${YACC_FILE}
	go generate ./pkg/filter

generate: ${YYPARSER}

# # # # # # # # # # # # # # # # # # # #
# Source and binary build / compile   #
# # # # # # # # # # # # # # # # # # # #
.PHONY: kubedump kubedump-server

all: docker charts kubedump

kubedump: bin/kubedump go.mod

bin/kubedump: ${SOURCES}
	${GO_BUILD} -o $@ ./pkg/cmd/kubedump

kubedump-server: bin/kubedump-server go.mod

bin/kubedump-server: ${SOURCES}
	${GO_BUILD} -o $@ ./pkg/cmd/kubedump-server

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
	   ${HELM_PACKAGE} --destination artifacts "${CHARTS_DIR}/$${chart}"; \
	done

# # # # # # # # # # # # # # # # # # # #
# Run go tests                        #
# # # # # # # # # # # # # # # # # # # #
unit: ${YYPARSER}
	${GO_TEST} ${UNIT_TEST_PATHS}

integration: ${YYPARSER}
	${GO_TEST} ${INTEGRATION_TEST_PATHS}

test: unit integration

# # # # # # # # # # # # # # # # # # # #
# Project management recipes          #
# # # # # # # # # # # # # # # # # # # #

.PHONY: clean fmt mostly-clean

mostly-clean:
	${RM} --recursive \
		kubedump-*.tar.gz \
		*.dump pkg/controller/*.dump tests/*.dump \
		tests/kubeconfig-* \
		pkg/filter/*.output pkg/filter/y.output

clean: mostly-clean
	${RM} --recursive artifacts bin release
