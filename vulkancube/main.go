package main

import (
	"log"

	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/android-go/android"
	"github.com/xlab/android-go/app"
)

func init() {
	app.SetLogTag("VulkanCube")
}

var appInfo = vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanCube\x00",
	PEngineName:        "golang\x00",
}

func main() {
	nativeWindowEvents := make(chan app.NativeWindowEvent)
	inputQueueEvents := make(chan app.InputQueueEvent, 1)
	inputQueueChan := make(chan *android.InputQueue, 1)

	app.Main(func(a app.NativeActivity) {
		// disable this to get the stack
		// defer catcher.Catch(
		// 	catcher.RecvLog(true),
		// 	catcher.RecvDie(-1),
		// )
		var demo Demo

		a.HandleNativeWindowEvents(nativeWindowEvents)
		a.HandleInputQueueEvents(inputQueueEvents)
		// just skip input events (so app won't be dead on touch input)
		go app.HandleInputQueues(inputQueueChan, func() {
			a.InputQueueHandled()
		}, app.SkipInputEvents)
		a.InitDone()
		for {
			select {
			case <-a.LifecycleEvents():
			// ignore
			case event := <-inputQueueEvents:
				switch event.Kind {
				case app.QueueCreated:
					inputQueueChan <- event.Queue
				case app.QueueDestroyed:
					inputQueueChan <- nil
				}
			case event := <-nativeWindowEvents:
				switch event.Kind {
				case app.NativeWindowCreated:
					err := vk.Init()
					orPanic(err)
					demo = NewDemoForAndroid(appInfo, event.Window)
					demo.InitModel()
					demo.Prepare(
						"shaders/cube-vert.spv",
						"shaders/cube-frag.spv",
						"assets/lunarg.ppm")

				case app.NativeWindowDestroyed:
					demo.Cleanup()
					vk.Destr
				case app.NativeWindowRedrawNeeded:
					demo.Step()
					log.Println("step done")
					a.NativeWindowRedrawDone()
				}
			}
		}
	})
}
