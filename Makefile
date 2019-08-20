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

favicon.go:
	@convert -background transparent -define icon:auto-resize=16 contrib/diecast-ico-source.svg contrib/favicon.ico
	@echo 'package diecast'                                                         > favicon.go
	@echo ''                                                                       >> favicon.go
	@echo 'func DefaultFavicon() []byte {'                                         >> favicon.go
	@echo '  // autogenerated from github.com/ghetzel/diecast:contrib/favicon.ico' >> favicon.go
	@echo '  return []byte{'                                                       >> favicon.go
	@hexdump -v -e '8/1 "0x%02x, " "\n"' contrib/favicon.ico | sed -e 's/0x  ,//g' >> favicon.go
	@echo '  }'                                                                    >> favicon.go
	@echo '}'                                                                      >> favicon.go
	@echo ''                                                                       >> favicon.go
	@gofmt -w favicon.go

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
