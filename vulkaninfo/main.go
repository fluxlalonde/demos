package main

import (
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/android-go/app"
	"github.com/xlab/catcher"
)

func init() {
	app.SetLogTag("VulkanInfo")
}

var appInfo = &vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanInfo\x00",
	PEngineName:        "golang\x00",
}

func main() {
	nativeWindowEvents := make(chan app.NativeWindowEvent)

	app.Main(func(a app.NativeActivity) {
		defer catcher.Catch(
			catcher.RecvLog(true),
			catcher.RecvDie(-1),
		)

		var vkDevice *VulkanDeviceInfo
		a.HandleNativeWindowEvents(nativeWindowEvents)
		a.InitDone()
		for {
			select {
			case <-a.LifecycleEvents():
				// ignore
			case event := <-nativeWindowEvents:
				switch event.Kind {
				case app.NativeWindowCreated:
					err := vk.Init()
					orPanic(err)
					vkDevice, err = NewVulkanDevice(appInfo, event.Window)
					orPanic(err)
					printInfo(vkDevice)
				case app.NativeWindowDestroyed:
					vkDevice.Destroy()
				case app.NativeWindowRedrawNeeded:
					a.NativeWindowRedrawDone()
				}
			}
		}
	})
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}
