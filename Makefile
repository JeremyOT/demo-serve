all: request serve

.PHONY: serve
serve:
	docker build -t jeremyot/serve serve

.PHONY: request
request:
	docker build -t jeremyot/request request