package main

import "unsafe"

type vkTexCubeUniform struct {
	mvp      [4][4]float32
	position [12 * 3][4]float32
	attr     [12 * 3][4]float32
}

const vkTexCubeFloats = 4*4 + 12*3*4 + 12*3*4

func (u *vkTexCubeUniform) Sizeof() int {
	return vkTexCubeFloats * 4
}

func (u *vkTexCubeUniform) Slice() []float32 {
	hdr := &sliceHeader{
		Len:  vkTexCubeFloats,
		Cap:  vkTexCubeFloats,
		Data: uintptr(unsafe.Pointer(u)),
	}
	return *(*[]float32)(unsafe.Pointer(hdr))
}

var g_vertex_buffer_data = []float32{
	-1, -1, -1, // -X side
	-1, -1, 1,
	-1, 1, 1,
	-1, 1, 1,
	-1, 1, -1,
	-1, -1, -1,

	-1, -1, -1, // -Z side
	1, 1, -1,
	1, -1, -1,
	-1, -1, -1,
	-1, 1, -1,
	1, 1, -1,

	-1, -1, -1, // -Y side
	1, -1, -1,
	1, -1, 1,
	-1, -1, -1,
	1, -1, 1,
	-1, -1, 1,

	-1, 1, -1, // +Y side
	-1, 1, 1,
	1, 1, 1,
	-1, 1, -1,
	1, 1, 1,
	1, 1, -1,

	1, 1, -1, // +X side
	1, 1, 1,
	1, -1, 1,
	1, -1, 1,
	1, -1, -1,
	1, 1, -1,

	-1, 1, 1, // +Z side
	-1, -1, 1,
	1, 1, 1,
	-1, -1, 1,
	1, -1, 1,
	1, 1, 1,
}

var g_uv_buffer_data = []float32{
	0, 0, // -X side
	1, 0,
	1, 1,
	1, 1,
	0, 1,
	0, 0,

	1, 0, // -Z side
	0, 1,
	0, 0,
	1, 0,
	1, 1,
	0, 1,

	1, 1, // -Y side
	1, 0,
	0, 0,
	1, 1,
	0, 0,
	0, 1,

	1, 1, // +Y side
	0, 1,
	0, 0,
	1, 1,
	0, 0,
	1, 0,

	1, 1, // +X side
	0, 1,
	0, 0,
	0, 0,
	1, 0,
	1, 1,

	0, 1, // +Z side
	0, 0,
	1, 1,
	0, 0,
	1, 0,
	1, 1,
}
