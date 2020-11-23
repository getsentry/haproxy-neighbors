BIN=haproxy-neighbors
DOCKER_TAG=us.gcr.io/sentryio/$(BIN)
VERSION=$(shell date +%Y%d%m)
REVISION:=0

$(BIN): src/haproxy_conf.go $(wildcard src/*.go)
	go build -ldflags="-s -w" -v -o $(BIN) ./src/...

src/haproxy_conf.go: haproxy.cfg gen.go
	go generate

docker: clean
	docker build --pull --rm -t $(BIN) .

tag: docker
	docker tag $(BIN) $(DOCKER_TAG):$(VERSION).$(REVISION)

publish: tag
	docker push $(DOCKER_TAG):$(VERSION).$(REVISION)

clean:
	rm -f $(BIN)

.PHONY: docker publish clean
