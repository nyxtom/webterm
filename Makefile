INSTALL_PATH ?= $(CURDIR)

$(./build_tool/build_config.sh build_config.mk $INSTALL_PATH)
$(shell ./bootstrap.sh >> /dev/null 2>&1)

export CGO_CFLAGS
export CGO_CXXFLAGS
export CGO_LDFLAGS
export LD_LIBRARY_PATH
export DYLD_LIBRARY_PATH
export GO_BUILD_TAGS

all: build

build:
	@go install --tags '$(GO_BUILD_TAGS)' ./...

clean:
	@go clean -i ./...

test:
	echo 'go test ./...'
	@go test -tags '$(GO_BUILD_TAGS)' ./...
