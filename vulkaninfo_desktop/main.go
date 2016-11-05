package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
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
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	win, err := glfw.CreateWindow(640, 480, "Vulkan Info", nil, nil)
	orPanic(err)

	err = vk.Init()
	orPanic(err)
	vkDevice, err := NewVulkanDevice(appInfo, win)
	orPanic(err)
	printInfo(vkDevice)
	vkDevice.Destroy()
}
