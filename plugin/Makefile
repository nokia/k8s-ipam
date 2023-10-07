IMAGE_NAME=k8s-ipam-cni
VERSION?=$(shell git describe --abbrev=0 | awk -F '.' 'BEGIN { OFS=FS  } {$$NF=$$NF+1; print $$0}')
BUILD := docker buildx build
COMMON_ARGS := --file=Dockerfile
COMMON_ARGS += --build-arg=VERSION=$(VERSION)
COMMON_ARGS += --build-arg=https_proxy=$(https_proxy)
COMMON_ARGS += --build-arg=http_proxy=$(https_proxy)


target-%: ## Builds specfied target with buildx
	@$(BUILD) \
  --target=$* \
	$(TARGET_ARGS) \
	$(COMMON_ARGS) \
  .

local-%:
	$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)" \

bin:
	make local-bin DEST=./
