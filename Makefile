BUILD != git describe --always --dirty --broken

.PHONY: serve
serve:
	BUILD=${BUILD} KO_DOCKER_REPO=jeremyot ko build ./serve --base-import-paths --tags ${BUILD},latest

.PHONY: request
request:
	BUILD=${BUILD} KO_DOCKER_REPO=jeremyot ko build ./request --base-import-paths --tags ${BUILD},latest

release: serve request
