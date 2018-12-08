.POSIX:
.SUFFIXES:
.PHONY: debug release vet clean version tools
.SILENT: version

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
DATAFILES = $(shell find data -type f)

BINARY=mailstats
FULL=github.com/stephane-martin/mailstats
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1.0
LDFLAGS=-ldflags '-X main.Version=${VERSION} -X main.GinMode=debug'
LDFLAGS_RELEASE=-ldflags '-w -s -X main.Version=${VERSION} -X main.GinMode=release'

debug: ${BINARY}_debug
release: ${BINARY}

vet:
	go vet ./...

clean:
	rm -f mailstats mailstats_debug

version:
	echo ${VERSION}

${BINARY}_debug: bindata.go models/incoming_gen.go ${SOURCES}
	go build -x -tags 'netgo osusergo' -o ${BINARY}_debug ${LDFLAGS} ${FULL}

${BINARY}: bindata.go models/incoming_gen.go ${SOURCES}
	go build -a -installsuffix nocgo -tags 'netgo osusergo' -o ${BINARY} ${LDFLAGS_RELEASE} ${FULL}

tools:
	go get -u github.com/kevinburke/go-bindata/go-bindata
	go get -u github.com/tinylib/msgp

bindata.go: ${DATAFILES}
	go-bindata data/

models/incoming_gen.go: models/incoming.go
	msgp -file models/incoming.go
