.POSIX:
.SUFFIXES:
.PHONY: debug release vet clean version
.SILENT: version

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
DATAFILES = $(shell find data -type f)

BINARY=mailstats
FULL=github.com/stephane-martin/mailstats
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

${BINARY}_debug: extractors/bindata.go models/incoming_gen.go ${SOURCES}
	go build -x -tags 'netgo osusergo' -o ${BINARY}_debug ${LDFLAGS} ${FULL}

${BINARY}: extractors/bindata.go models/incoming_gen.go ${SOURCES}
	go build -a -installsuffix nocgo -tags 'netgo osusergo' -o ${BINARY} ${LDFLAGS_RELEASE} ${FULL}

retool:
	go get -u github.com/twitchtv/retool
	cp ${GOPATH}/bin/retool .

.tools_sync: retool tools.json
	./retool sync
	touch .tools_sync

extractors/bindata.go: .tools_sync ${DATAFILES}
	./retool do go-bindata -pkg extractors -o extractors/bindata.go data/

models/incoming_gen.go: .tools_sync models/incoming.go
	./retool do msgp -file models/incoming.go
