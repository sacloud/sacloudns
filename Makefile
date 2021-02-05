VERSION=0.0.1
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"
GO111MODULE=on

all: sacloudns

.PHONY: sacloudns

sacloudns: main.go
	go build $(LDFLAGS) -o sacloudns

linux: main.go
	GOOS=linux GOARCH=linux go build $(LDFLAGS) -o sacloudns

clean:
	rm -rf sacloudns

check:
	go test ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin main
