.PHONY: all portal agent web clean proto

all: portal agent

web:
	cd web && npm run build
	rm -rf cmd/portal/web/dist
	mkdir -p cmd/portal/web/dist
	cp -r web/dist/* cmd/portal/web/dist/

portal: web
	go build -o bin/rtbh-portal ./cmd/portal/

agent:
	go build -o bin/rtbhd ./cmd/agent/

proto:
	buf generate proto

clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf cmd/portal/web/dist/

test:
	go test ./...
