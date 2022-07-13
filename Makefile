BUILD != git describe --always --dirty --broken

all: request serve

.PHONY: serve
serve:
	docker buildx build --platform=linux/amd64 -t jeremyot/serve:latest -t jeremyot/serve:$(BUILD) --build-arg BUILD=$(BUILD) --load serve

.PHONY: request
request:
	docker buildx build --platform=linux/amd64 -t jeremyot/request:latest -t jeremyot/request:$(BUILD) --build-arg BUILD=$(BUILD) --load request

release: serve request
	docker buildx build --platform=linux/amd64,linux/arm64 -t jeremyot/serve:latest -t jeremyot/serve:$(BUILD) --build-arg BUILD=$(BUILD) --push serve
	docker buildx build --platform=linux/amd64,linux/arm64 -t jeremyot/request:latest -t jeremyot/request:$(BUILD) --build-arg BUILD=$(BUILD) --push request
	docker buildx build --platform=linux/amd64,linux/arm64 -t jeremyot/serve:latest -t jeremyot/serve:latest --build-arg BUILD=$(BUILD) --push serve
	docker buildx build --platform=linux/amd64,linux/arm64 -t jeremyot/request:latest -t jeremyot/request:latest --build-arg BUILD=$(BUILD) --push request
