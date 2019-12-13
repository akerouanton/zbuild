export GO111MODULE=on

GO_BUILD_STATIC := go build -ldflags '-extldflags "-fno-PIC -static"' -buildmode pie -tags 'osusergo netgo static_build'

# Either use `gotest` if available (same as `go test` but with colors), or use
# `go test`.
GOTEST := go test
ifneq ($(shell which gotest),)
	GOTEST := gotest
endif

ifeq ($(IMAGE_TAG),GIT_SHA)
	IMAGE_TAG := $(shell git rev-parse --short HEAD)
endif

.PHONY: build
build:
	$(GO_BUILD_STATIC) -o bin/zbuild ./cmd/zbuild
	$(GO_BUILD_STATIC) -o bin/zbuilder ./cmd/zbuilder

.PHONY: test
test:
	$(GOTEST) -v -cover -coverprofile cover.out ./...
	go tool cover -o cover.html -html=cover.out

.PHONY: gen-mocks
gen-mocks:
	go generate ./...

.PHONY: gen-testdata
gen-testdata:
	@$(GOTEST) -v ./pkg/llbutils -testdata
	@$(GOTEST) -v ./pkg/defkinds/php -testdata
	@$(GOTEST) -v ./pkg/defkinds/webserver -testdata
	@echo "WARNING: Be sure to review regenerated testdata files before committing them."

.PHONY: build-image
build-image: .validate-image-tag build
	docker build -t akerouanton/zbuilder:$(IMAGE_TAG) -f Dockerfile.builder bin/

.PHONY: push
push: .validate-image-tag
	docker push akerouanton/zbuilder:$(IMAGE_TAG)

.PHONY: install
install:
	cp bin/zbuild ~/go/bin


####################
## Preconditions
####################


.PHONY: .validate-image-tag
.validate-image-tag:
ifeq ($(IMAGE_TAG),)
	$(error You have to provide an IMAGE_TAG)
endif
