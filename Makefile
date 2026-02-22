APP     := yt-dlp-web
DIST    := dist
LDFLAGS := -s -w
GOFLAGS := CGO_ENABLED=0

PLATFORMS := \
	windows/amd64/.exe \
	windows/386/.exe \
	windows/arm64/.exe \
	darwin/amd64/ \
	darwin/arm64/

.PHONY: build clean release

build:
	go build -ldflags="$(LDFLAGS)" -trimpath -o $(DIST)/$(APP) .

clean:
	rm -rf $(DIST)

release: clean
	@mkdir -p $(DIST)
	@$(foreach platform,$(PLATFORMS),\
		$(eval GOOS := $(word 1,$(subst /, ,$(platform))))\
		$(eval GOARCH := $(word 2,$(subst /, ,$(platform))))\
		$(eval EXT := $(word 3,$(subst /, ,$(platform))))\
		$(eval OUT := $(DIST)/$(APP)-$(GOOS)-$(GOARCH)$(EXT))\
		echo "Building $(OUT)" && \
		$(GOFLAGS) GOOS=$(GOOS) GOARCH=$(GOARCH) \
			go build -ldflags="$(LDFLAGS)" -trimpath -o $(OUT) . && \
	) echo "Done"
