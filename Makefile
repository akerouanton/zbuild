export GO111MODULE=on

GO_BUILD_STATIC := go build -ldflags '-extldflags "-fno-PIC -static"' -buildmode pie -tags 'osusergo netgo static_build'

# Either use `gotest` if available (same as `go test` but with colors), or use
# `go test`.
GOTEST := go test
ifneq ($(shell which gotest),)
	GOTEST := gotest
endif

.PHONY: build
build:
	$(GO_BUILD_STATIC) -o bin/webdf ./cmd/webdf
	$(GO_BUILD_STATIC) -o bin/webdf-builder ./cmd/webdf-builder

.PHONY: test
test:
	$(GOTEST) -v -cover -coverprofile cover.out ./...
	go tool cover -o cover.html -html=cover.out

.PHONY: gen-mocks
gen-mocks:
	go generate ./...

.PHONY: gen-testdata
gen-testdata:
	@$(GOTEST) ./pkg/llbutils -testdata
	@echo "WARNING: Be sure to review regenerated testdata files before committing them."

.PHONY: build-image
build-image: .validate-image-tag build
	docker build -t akerouanton/webdf-builder:$(IMAGE_TAG) -f Dockerfile.builder bin/

.PHONY: push
push: .validate-image-tag
	docker push akerouanton/webdf-builder:$(IMAGE_TAG)

.PHONY: install
install:
	cp webdf ~/go/bin


####################
## Preconditions
####################


.PHONY: .validate-image-tag
.validate-image-tag:
ifeq ($(IMAGE_TAG),)
	$(error You have to provide an IMAGE_TAG)
endif
