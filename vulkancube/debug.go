package main

import (
	"log"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

// dbgCallbackFunc is a Go alternative to the vk.DebugReportCallbackAndroid helper.
func dbgCallbackFunc(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {

	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		log.Printf("[Layer %s][ERROR %d] %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		log.Printf("[Layer %s][WARN %d] %s", pLayerPrefix, messageCode, pMessage)
	default:
		log.Printf("[Layer %s][WARN] unknown debug message %d", pLayerPrefix, messageCode)
	}
	// Returning false tells the layer not to stop when the event occurs, so
	// they see the same behavior with and without validation layers enabled.
	return vk.Bool32(vk.False)
}
