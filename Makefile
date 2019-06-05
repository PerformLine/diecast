.PHONY: test deps docs
.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on
LOCALS      := $(shell find . -type f -name '*.go')
BIN         ?= diecast-$(shell go env GOOS)-$(shell go env GOARCH)

all: deps test build docs

deps:
	go get ./...
	-go mod tidy

fmt:
	go generate -x ./...
	gofmt -w $(LOCALS)
	go vet ./...

test:
	go test -count=1 ./...

build: fmt
	go build --ldflags '-extldflags "-static"' -installsuffix cgo -ldflags '-s' -o bin/$(BIN) cmd/diecast/main.go
	#GOOS=darwin go build --ldflags '-extldflags "-static"' -installsuffix cgo -ldflags '-s' -o bin/diecast-darwin-amd64 cmd/diecast/main.go
	which diecast && cp -v bin/$(BIN) $(shell which diecast) || true

docs:
	cd docs && make

package:
	-rm -rf pkg
	mkdir -p pkg/usr/bin
	cp bin/$(BIN) pkg/usr/bin/diecast
	fpm \
		--input-type  dir \
		--output-type deb \
		--deb-user    root \
		--deb-group   root \
		--name        diecast \
		--version     `./pkg/usr/bin/diecast -v | cut -d' ' -f3` \
		-C            pkg
