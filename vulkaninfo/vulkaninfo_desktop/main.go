package main

import (
	"github.com/golang-ui/glfw"
	"github.com/vulkan-go/demos/vulkaninfo"
	vk "github.com/vulkan-go/vulkan"
)

var appInfo = &vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanInfo\x00",
	PEngineName:        "golang\x00",
}

func main() {
	orPanic(glfw.Init())
	orPanic(vk.Init())

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	win := glfw.CreateWindow(640, 480, "Vulkan Info", nil, nil)
	orPanic(win != nil)
	defer glfw.DestroyWindow(win)

	vkDevice, err := vulkaninfo.NewVulkanDevice(appInfo, win.Ptr())
	orPanic(err)
	vulkaninfo.PrintInfo(vkDevice)
	vkDevice.Destroy()
}

func orPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(err)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	}
}
