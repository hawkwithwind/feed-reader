GOOS=linux
GOARCH=amd64

EXECUTABLE=feed-reader
RUNTIME_PATH=build
RUNTIME_IMAGE=feed-reader
PACKAGE=github.com/hawkwithwind/$(EXECUTABLE)
PROTOC_PATH=/opt/programs/protoc/bin

GOIMAGE=golang:1.15-alpine3.12

SOURCES=$(shell find server -type f -name '*.go' -not -path "./vendor/*")
BASE=$(GOPATH)/src/$(PACKAGE)

#DBPATH="$(feeddb)"


$(RUNTIME_PATH)/$(EXECUTABLE): $(SOURCES) $(RUNTIME_PATH) build-golang-image build-migrate-image
	docker run --rm \
        --env HTTPS_PROXY=$(https_proxy) \
        --env HTTP_PROXY=$(http_proxy) \
        --net=host \
	-v $(GOPATH)/src:/go/src \
	-v $(GOPATH)/pkg:/go/pkg \
	-v $(shell pwd)/$(RUNTIME_PATH):/go/bin/${GOOS}_${GOARCH} \
	-e GOOS=$(GOOS) -e GOARCH=$(GOARCH) -e GOBIN=/go/bin/$(GOOS)_$(GOARCH) -e CGO_ENABLED=0 \
	$(RUNTIME_IMAGE):build-golang go get -a -installsuffix cgo $(PACKAGE)/server/...

#$(RUNTIME_PATH)/dockerfile: runtime-image
#	sh make_docker_file.sh $(RUNTIME_PATH)/Dockerfile $(RUNTIME_IMAGE) $(EXECUTABLE)

#runtime-image:
#	docker build -t $(RUNTIME_IMAGE) docker/runtime

build-migrate-image:
	docker build --build-arg mirror=$(alpine_mirror) -t $(RUNTIME_IMAGE):migrate docker/migrate

build-nodejs-image:
	docker build --build-arg mirror=$(debian_mirror) -t $(RUNTIME_IMAGE):build-nodejs docker/build/nodejs

build-golang-image:
	docker build --build-arg mirror=$(alpine_mirror) -t $(RUNTIME_IMAGE):build-golang docker/build/golang

$(RUNTIME_PATH):
	[ -d $(RUNTIME_PATH) ] || mkdir $(RUNTIME_PATH) && \
	[ -d $(RUNTIME_PATH)/static ] || mkdir $(RUNTIME_PATH)/static

clean:
	if [ -d $(RUNTIME_PATH)/static/ ]; then chmod -R +w $(RUNTIME_PATH)/static/ ; fi && \
	rm -rf $(RUNTIME_PATH)

fmt:
	docker run --rm \
	-v $(shell pwd):/go/src/$(PACKAGE) \
	$(RUNTIME_IMAGE):build-golang sh -c "cd /go/src/$(PACKAGE)/ && gofmt -l -w $(SOURCES)"

test: $(SOURCES) $(RUNTIME_PATH) build-golang-image
	docker run --rm \
        -e HTTPS_PROXY=$(https_proxy) \
        -e HTTP_PROXY=$(http_proxy) \
        -e DBPATH="$(TESTDBPATH)" \
        --net=host \
	-v $(GOPATH)/src:/go/src \
	-v $(GOPATH)/pkg:/go/pkg \
	-v $(shell pwd)/$(RUNTIME_PATH):/go/bin/${GOOS}_${GOARCH} \
	-e GOOS=$(GOOS) -e GOARCH=$(GOARCH) -e GOBIN=/go/bin/$(GOOS)_$(GOARCH) -e CGO_ENABLED=0 \
	$(RUNTIME_IMAGE):build-golang sh -c "cd /go/src/$(PACKAGE)/server/ && go test -v ./..."

cgo: $(RUNTIME_PATH)/$(EXECUTABLE)

npm-audit-fix: build-nodejs-image
	docker run --rm \
	-v $(shell pwd)/frontend:/home/work \
	$(RUNTIME_IMAGE):build-nodejs npm audit fix

.PHONY: cgo




