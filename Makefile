.PHONY: build
build: .validate-image-tag
	go build -o webdf ./cmd/webdf
	go build -o webdf-builder ./cmd/webdf-builder
	docker build -t akerouanton/webdf-builder:$(IMAGE_TAG) -f Dockerfile.builder .

.PHONY: push
push: .validate-image-tag
	docker push akerouanton/webdf-builder:$(IMAGE_TAG)

.PHONY: install
install:
	cp webdf ~/go/bin

.PHONY: .validate-image-tag
.validate-image-tag:
ifeq ($(IMAGE_TAG),)
	$(error You have to provide an IMAGE_TAG)
endif
