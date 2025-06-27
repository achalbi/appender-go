.PHONY: build test docker-build docker-push

APP_NAME = appender-go
DOCKER_REGISTRY = your-docker-registry.example.com
DOCKER_IMAGE = $(DOCKER_REGISTRY)/$(APP_NAME)
VERSION ?= latest

build:
	go build -o $(APP_NAME) main.go

test:
	go test ./...

docker-build: build
	docker build -t $(DOCKER_IMAGE):$(VERSION) .

docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)