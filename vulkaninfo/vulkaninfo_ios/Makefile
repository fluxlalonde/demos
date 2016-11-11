WORK ?= $(shell pwd)
PACKAGE ?= github.com/vulkan-go/demos/vulkaninfo/vulkaninfo_ios
MOLTEN_FW ?= /Users/xlab/Documents/dev/Molten-0.13.0/MoltenVK/iOS/

CC ?= $(shell xcrun --sdk iphoneos --find clang)
CXX ?= $(shell xcrun --sdk iphoneos --find clang)
ISYSROOT ?= $(shell xcrun --sdk iphoneos --show-sdk-path)

all: build

build:
	mkdir -p $(WORK)/build
	GOOS=darwin GOARCH=arm64 CC=$(CC) CXX=$(CXX) \
		CGO_CFLAGS="-isysroot $(ISYSROOT) -miphoneos-version-min=8.0 -arch arm64" \
		CGO_LDFLAGS="-isysroot $(ISYSROOT) -F$(MOLTEN_FW) -miphoneos-version-min=8.0 -arch arm64" \
		CGO_ENABLED=1 go build -tags="ios" -o=$(WORK)/main/main $(PACKAGE)
	xcrun xcodebuild -configuration Release -project $(WORK)/main.xcodeproj

install:
	# https://github.com/phonegap/ios-deploy
	ios-deploy -b $(WORK)/build/Release-iphoneos/main.app

debug:
	ios-deploy -d -b $(WORK)/build/Release-iphoneos/main.app

clean:
	rm -rf $(WORK)/build
	rm -f $(WORK)/main/main

.PHONY: build