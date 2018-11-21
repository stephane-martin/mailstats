.POSIX:
.SUFFIXES:
.PHONY: debug release vet clean version
.SILENT: version

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

BINARY=mailstats
FULL=github.com/stephane-martin/mailstats
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1.0
LDFLAGS=-ldflags '-X main.Version=${VERSION}'
LDFLAGS_RELEASE=-ldflags '-w -s -X main.Version=${VERSION}'

debug: ${BINARY}_debug
release: ${BINARY}

vet:
	go vet ./...

clean:
	rm -f mailstats mailstats_debug

version:
	echo ${VERSION}

${BINARY}_debug: ${SOURCES} 
	go build -x -tags netgo -o ${BINARY}_debug ${LDFLAGS} ${FULL}

${BINARY}: ${SOURCES} 
	go build -a -installsuffix nocgo -tags netgo -o ${BINARY} ${LDFLAGS_RELEASE} ${FULL}


