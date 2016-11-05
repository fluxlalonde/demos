package main

import vk "github.com/vulkan-go/vulkan"

var appInfo = &vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanInfo\x00",
	PEngineName:        "golang\x00",
}

func main() {
	err := vk.Init()
	orPanic(err)
	vkDevice, err := NewVulkanDevice(appInfo)
	orPanic(err)
	printInfo(vkDevice)
	vkDevice.Destroy()
}
