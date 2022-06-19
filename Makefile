#SOURCES=./pkg/*.go
SOURCES=
TEST_PATHS=./pkg

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
.PHONY: aqueduct

kdump: bin/kdump

bin/kdump: ${SOURCES} cmd/kdump/main.go
	${GO_BUILD} -o $@ ./cmd/kdump

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
	make --directory examples mostly-clean

clean: mostly-clean
	${RM} --recursive bin
	make --directory examples clean

fmt:
	${GO_FMT} ./cmd/kdump
