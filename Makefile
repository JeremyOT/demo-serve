BUILD != git describe --always --dirty --broken

all: request serve

.PHONY: serve
serve:
	docker build -t jeremyot/serve:latest -t jeremyot/serve:$(BUILD) --build-arg BUILD=$(BUILD) serve

.PHONY: request
request:
	docker build -t jeremyot/request:latest -t jeremyot/request:$(BUILD) --build-arg BUILD=$(BUILD) request

release: serve request
	docker push jeremyot/request:$(BUILD)
	docker push jeremyot/request:latest
	docker push jeremyot/serve:$(BUILD)
	docker push jeremyot/serve:latest