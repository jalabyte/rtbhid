.PHONY: all portal agent web clean proto portal-linux agent-linux all-linux

all: portal agent

all-linux: portal-linux agent-linux

web:
	cd web && npm run build
	rm -rf cmd/portal/web/dist
	mkdir -p cmd/portal/web/dist
	cp -r web/dist/* cmd/portal/web/dist/

portal: web
	go build -o bin/rtbh-portal ./cmd/portal/

agent:
	go build -o bin/rtbhd ./cmd/agent/

portal-linux: web
	GOOS=linux GOARCH=amd64 go build -o bin/rtbh-portal-linux-amd64 ./cmd/portal/

agent-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/rtbhd-linux-amd64 ./cmd/agent/

proto:
	buf generate proto
	mv gen/*.go gen/rtbhid/

clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf cmd/portal/web/dist/

test:
	go test ./...
