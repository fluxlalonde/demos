LOCAL_PATH := $(call my-dir)

include $(CLEAR_VARS)

LOCAL_MODULE    := example
LOCAL_SRC_FILES := lib/libvulkaninfo.so
LOCAL_LDLIBS    := -llog -landroid

include $(PREBUILT_SHARED_LIBRARY)
