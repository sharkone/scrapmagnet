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
DOCKER_IMAGE = sharkone/libtorrent-go

all: build

build:
	for i in $(PLATFORMS); do 																													\
		$(DOCKER) run -ti --rm -v $(HOME):$(HOME) -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) $(DOCKER_IMAGE):$$i make cc-build || exit 1;	\
	done

###############################################################################
# Cross-compilation environment (inside each Docker image)
###############################################################################
ifeq ($(CROSS_GOOS), windows)
OUT_NAME = $(NAME).exe
else
OUT_NAME = $(NAME)
endif

cc-build:
	mkdir -p $(CROSS_GOOS)-$(CROSS_GOARCH)
	GOOS=$(CROSS_GOOS) GOARCH=$(CROSS_GOARCH) GOARM=$(CROSS_GOARM) go build -o $(CROSS_GOOS)-$(CROSS_GOARCH)/$(OUT_NAME)

cc-run:
	$(CROSS_GOOS)-$(CROSS_GOARCH)/$(OUT_NAME)
