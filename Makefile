GOCMD=go
GOTEST=$(GOCMD) test
GOCOVER=$(GOCMD) tool cover
GOFMT=gofmt
DOCKER=docker
DOCKER_REPO=dockerbar
APP_NAME=image-previewer
APP_VERSION=$(shell git tag | tail -1)
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

.DEFAULT_GOAL := all
.PHONY: all test test-without-mocks coverage check-fmt fmt

all: check-fmt test coverage

test:
	$(GOTEST) -v ./... -covermode=count -coverprofile=c.out

test-without-mocks:
	GONOMOCKS=1 $(GOTEST) -v ./... -covermode=count -coverprofile=c.out

test-intergration:
	./integration_test.sh

coverage:
	$(GOCOVER) -func=c.out

check-fmt:
	$(GOFMT) -d ${GOFILES}

fmt:
	$(GOFMT) -w ${GOFILES}

lint:
	@echo 'should start the linter'
	
run:
	$(GOCMD) run cmd/${APP_NAME}/main.go	

build:
	@echo 'build the image $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION)'
	${DOCKER} build -t $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION) .

	@echo 'Result image $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION)'

docker-build:
	@echo 'build the image $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION)'
	${DOCKER} build -t $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION) .
    
docker-push:
	@echo 'push the image $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION)'
	${DOCKER} push $(DOCKER_REPO)/$(APP_NAME):$(APP_VERSION)

docker-build-and-push: docker-build docker-push
	