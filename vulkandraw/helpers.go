package main

import (
	"log"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

func check(ret vk.Result, name string) bool {
	if err := vk.Error(ret); err != nil {
		log.Println("[WARN]", name, "failed with", err)
		return true
	}
	return false
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func repackUint32(data []byte) []uint32 {
	buf := make([]uint32, len(data)/4)
	hdr := (*sliceHeader)(unsafe.Pointer(&buf))
	vk.MemCopyByte(unsafe.Pointer(hdr.Data), data)
	return buf
}

type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

const MaxUint32 = 1<<32 - 1 // or ^uint32(0)
