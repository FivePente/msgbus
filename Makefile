.PHONY: dev build image test deps clean

CGO_ENABLED=0
COMMIT=`git rev-parse --short HEAD`
LIBRARY=msgbus
SERVER=msgbusd
CLIENT=msgbus
REPO?=prologic/$(LIBRARY)
TAG?=latest
BUILD?=-dev

BUILD_TAGS="netgo static_build"
BUILD_LDFLAGS="-w -X github.com/$(REPO).GitCommit=$(COMMIT) -X github.com/$(REPO)/Build=$(BUILD)"

all: dev

dev: build
	@./cmd/$(SERVER)/$(SERVER)

deps:
	@go get ./...

build: clean deps
	@echo " -> Building $(SERVER) $(TAG)$(BUILD) ..."
	@cd cmd/$(SERVER) && \
		go build -tags $(BUILD_TAGS) -installsuffix netgo \
		-ldflags $(BUILD_LDFLAGS) .
	@echo "Built $$(./cmd/$(SERVER)/$(SERVER) -v)"
	@echo
	@echo " -> Building $(CLIENT) $(TAG)$(BUILD) ..."
	@cd cmd/$(CLIENT) && \
		go build -tags $(BUILD_TAGS) -installsuffix netgo \
		-ldflags $(BUILD_LDFLAGS) .
	@echo "Built $$(./cmd/$(CLIENT)/$(CLIENT) -v)"

image:
	@docker build --build-arg TAG=$(TAG) --build-arg BUILD=$(BUILD) -t $(REPO):$(TAG) .
	@echo "Image created: $(REPO):$(TAG)"

test:
	@go test -v -cover -race $(TEST_ARGS)

clean:
	@rm -rf $(APP)
