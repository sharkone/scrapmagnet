###############################################################################
# Common
###############################################################################
NAME = scrapmagnet

###############################################################################
# Development environment
###############################################################################
PLATFORMS = android-arm		\
			darwin-x64 		\
			linux-x86 		\
			linux-x64 		\
			linux-arm 		\
			windows-x86 	\
			windows-x64

DOCKER 		 = docker
DOCKER_IMAGE = steeve/libtorrent-go

all: build

build:
	for i in $(PLATFORMS); do 																													\
		$(DOCKER) run -ti --rm -v $(HOME):$(HOME) -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) $(DOCKER_IMAGE):$$i make cc-build || exit 1;	\
	done

run: build
	for i in $(PLATFORMS); do 																																\
		$(DOCKER) run -ti --rm -v $(HOME):$(HOME) -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) -p 8080:8080 $(DOCKER_IMAGE):$$i make cc-run || exit 1;	\
	done

###############################################################################
# Cross-compilation environment (inside each Docker image)
###############################################################################
cc-build:
	GOOS=$(CROSS_GOOS) GOARCH=$(CROSS_GOARCH) GOARM=$(CROSS_GOARM) go build -o $(NAME)_$(CROSS_GOOS)-$(CROSS_GOARCH)

cc-run:
	./$(NAME)_$(CROSS_GOOS)-$(CROSS_GOARCH)
