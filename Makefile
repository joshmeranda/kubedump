SOURCES=./pkg/cmd/*.go ./pkg/cmd/kubedump/*.go ./pkg/*.go ./pkg/filter/*.go ./pkg/controller/*.go

UNIT_TEST_PATHS=./pkg/filter ./pkg/controller ./pkg/cmd
INTEGRATION_TEST_PATHS=./tests
TEST_PATHS=${UNIT_TEST_PATHS} ${INTEGRATION_TEST_PATHS}

KUBEDUMP_VERSION=$(shell tools/version.bash get)

# # # # # # # # # # # # # # # # # # # #
# Go commands                         #
# # # # # # # # # # # # # # # # # # # #
GO_BUILD=go build -ldflags "-X kubedump/pkg/cmd.Version=${KUBEDUMP_VERSION}" -race
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
	@echo "  all                build all binaries and  run code generation"
	@echo "  generate           run code generation"
	@echo "  kubedump           build the kubedump binary"
	@echo  "  unit              	run unit tests"
	@echo  "  integration       run integration tests"
	@echo "  test               run all tests"
	@echo "  mostly-clean       clean any project generated files (not-including deliverables)"
	@echo "  clean              clean built and generated files"
	@echo "  fmt                run the source through the builtin go formatter"
	@echo ""
	@echo "Values:"
	@echo "  VERBOSE            if set various recipes are run with verbose output"

all: fmt kubedump generate

# # # # # # # # # # # # # # # # # # # #
# code generation                     #
# # # # # # # # # # # # # # # # # # # #
YYPARSER=pkg/filter/yyparser.go
YACC_FILE=pkg/codegen/parser.y

.PHONY: generate

${YYPARSER}: ${YACC_FILE}
	go generate ./pkg/filter

generate: ${YYPARSER}

# # # # # # # # # # # # # # # # # # # #
# Source and binary build / compile   #
# # # # # # # # # # # # # # # # # # # #
.PHONY: kubedump

kubedump: bin/kubedump go.mod

bin/kubedump: ${SOURCES}
	${GO_BUILD} -o $@ ./pkg/cmd/kubedump

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
		*.dump pkg/controller/*.dump tests/*.dump pkg/cmd/*.dump \
		tests/kubeconfig-* \
		pkg/filter/*.output pkg/filter/y.output

clean: mostly-clean
	${RM} --recursive artifacts bin release
