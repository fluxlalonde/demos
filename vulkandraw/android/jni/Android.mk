LOCAL_PATH := $(call my-dir)

include $(CLEAR_VARS)

LOCAL_MODULE    := vulkandraw
LOCAL_SRC_FILES := lib/libvulkandraw.so
LOCAL_LDLIBS    := -llog -landroid

include $(PREBUILT_SHARED_LIBRARY)
