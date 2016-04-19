all: vendor fmt build

update:
	glide up --strip-vcs --update-vendored

vendor:
	go list github.com/Masterminds/glide
	glide install --strip-vcs --update-vendored

fmt:
	gofmt -w .

build:
	go build -o bin/`basename ${PWD}`
