package main

import (
	"log"

	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/android-go/android"
	"github.com/xlab/android-go/app"
	"github.com/xlab/catcher"
)

func init() {
	app.SetLogTag("VulkanDraw")
}

var appInfo = vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanDraw\x00",
	PEngineName:        "golang\x00",
}

func main() {
	nativeWindowEvents := make(chan app.NativeWindowEvent)
	inputQueueEvents := make(chan app.InputQueueEvent, 1)
	inputQueueChan := make(chan *android.InputQueue, 1)

	app.Main(func(a app.NativeActivity) {
		// disable this to get the stack
		defer catcher.Catch(
			catcher.RecvLog(true),
			catcher.RecvDie(-1),
		)
		var (
			v   VulkanDeviceInfo
			s   VulkanSwapchainInfo
			r   VulkanRenderInfo
			b   VulkanBufferInfo
			gfx VulkanGfxPipelineInfo

			vkActive bool
		)

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
					v, err = NewVulkanDeviceAndroid(appInfo, event.Window)
					orPanic(err)
					s, err = v.CreateSwapchain()
					orPanic(err)
					r, err = CreateRenderer(v.device, s.displayFormat)
					orPanic(err)
					err = s.CreateFramebuffers(r.renderPass, vk.NullHandle)
					orPanic(err)
					b, err = v.CreateBuffers()
					orPanic(err)
					gfx, err = CreateGraphicsPipeline(v.device, s.displaySize, r.renderPass)
					orPanic(err)
					log.Println("[INFO] swapchain lengths:", s.swapchainLen)
					err = r.CreateCommandBuffers(s.DefaultSwapchainLen())
					orPanic(err)

					VulkanInit(&v, &s, &r, &b, &gfx)
					vkActive = true

				case app.NativeWindowDestroyed:
					vkActive = false
					DestroyInOrder(&v, &s, &r, &b, &gfx)
				case app.NativeWindowRedrawNeeded:
					if vkActive {
						VulkanDrawFrame(v, s, r)
					}
					a.NativeWindowRedrawDone()
				}
			}
		}
	})
}
