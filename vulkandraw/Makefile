ANDROID_TOOLCHAIN_DIR ?= $(shell pwd)/toolchain
ANDROID_API ?= 21
ANDROID_SYSROOT = $(NDK)/platforms/android-$(ANDROID_API)/arch-arm

all: toolchain build apk

toolchain:
	$(NDK)/build/tools/make_standalone_toolchain.py \
		--api=$(ANDROID_API) --install-dir=$(ANDROID_TOOLCHAIN_DIR) \
		--arch=arm --stl libc++

build:
	mkdir -p android/jni/lib
	CC="$(ANDROID_TOOLCHAIN_DIR)/bin/arm-linux-androideabi-gcc" \
	CXX="$(ANDROID_TOOLCHAIN_DIR)/bin/arm-linux-androideabi-g++" \
	CGO_CFLAGS="-march=armv7-a" \
	GOOS=android \
	GOARCH=arm \
	GOARM=7 \
	CGO_ENABLED=1 \
	go build -buildmode=c-shared -o android/jni/lib/libvulkandraw.so
 
apk:
	cd android && make

clean:
	cd android && make clean

install:
	cd android && make install

listen:
	adb logcat -c
	adb logcat *:S VulkanDraw

.PHONY: shaders

shaders:
	# Obtain glslangValidator at https://github.com/google/shaderc
	glslangValidator -s -V -o shaders/tri-vert.spv shaders/tri.vert
	glslangValidator -s -V -o shaders/tri-frag.spv shaders/tri.frag
	go get github.com/jteeuwen/go-bindata
	go-bindata -pkg main shaders/
