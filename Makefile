.PHONY: test deps docs
.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on
LOCALS      := $(shell find . -type f -name '*.go')
BIN         ?= diecast-$(shell go env GOOS)-$(shell go env GOARCH)
VERSION      = $(grep 'const ApplicationVersion' version.go | cut -d= -f2 | tr -d '`' | tr -d ' ')

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
#	cd tests/render && make

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
	CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' -installsuffix cgo -ldflags '-s' -o bin/$(BIN)-nocgo cmd/diecast/main.go
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

clients.csr:
	openssl req -new -newkey rsa:4096 -nodes -keyout clients.key -out clients.csr -subj '/O=ghetzel/CN=diecast'

clients.crt: clients.csr
	openssl x509 -req -days 3650 -in clients.csr -signkey clients.key -out clients.crt

sign-client: clients.crt
	test -n "$(NAME)" || $(error NAME (client name) is not set)
#	Generate new client private key and signing request (CSR)
	openssl req \
		-new \
		-nodes \
		-keyout $(NAME).key \
		-out $(NAME).csr \
		-days 3650 \
		-subj '/O=diecast/CN=$(NAME)' \
		-extensions v3_req

#	Sign client certificate with clients CA
	openssl x509 \
		-req \
		-days 3650 \
		-in $(NAME).csr \
		-CA clients.crt \
		-CAkey clients.key \
		-CAcreateserial \
		-out $(NAME).crt \
		-extensions v3_req

#	Also generate a PKCS#12 encoded bundle of the newly generated cert+key
	openssl pkcs12 \
		-export \
		-in $(NAME).crt \
		-inkey $(NAME).key \
		-out $(NAME).p12

docker:
	@echo "Building Docker image for v$(VERSION)"
	docker build -t ghetzel/diecast:$(VERSION) .
	docker tag ghetzel/diecast:$(VERSION) ghetzel/diecast:latest
	docker push ghetzel/diecast:$(VERSION)
	docker push ghetzel/diecast:latest
