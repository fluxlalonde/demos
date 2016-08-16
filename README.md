Golang Vulkan API Demos
=======================

This repository contains demos made with [Vulkan API for Golang](http://github.com/vulkan-go/vulkan). Currently these are Android apps only, but I hope for contributions.

## Initial setup

Refer to [github.com/xlab/android-go/example#prerequisites](https://github.com/xlab/android-go/tree/master/example#prerequisites) for the first run instructions for Android NDK. Please note that you'll need to obtain a device with native Vulkan API support.

Once setup correctly, this course of actions is the flow of building and debugging of any app:

```bash
make # you can debug any bugs in Go code here
make install
make listen
```

After you make changes, a simple `make && make install` would be enough to have your app APK updated on the device.
See the [Makefile](/Makefile) for details on each step.

## [VulkanInfo](https://github.com/vulkan-go/demos/vulkaninfo)

This is a simple app template, ported from [googlesamples/android-vulkan-tutorials/tutorial01_load_vulkan](https://github.com/googlesamples/android-vulkan-tutorials). VulkanInfo gathers info about your Vulkan environment's properties and prints them to the log. After you install and run the target app, check out the ADB logcat logs:

```
$ make listen
adb logcat -c
adb logcat *:S VulkanInfo
--------- beginning of system
--------- beginning of main
08-16 21:22:21.008  5096  5111 I VulkanInfo: ╭─────────────────────────────────────────────────╮
08-16 21:22:21.008  5096  5111 I VulkanInfo: │    VULKAN PROPERTIES AND SURFACE CAPABILITES    │
08-16 21:22:21.008  5096  5111 I VulkanInfo: ├────────────────────────┬────────────────────────┤
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Physical Device Name   │ NVIDIA Tegra K1        │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Physical Device Vendor │ 10de                   │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Physical Device Type   │ Integrated GPU         │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Physical GPUs          │ 1                      │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ API Version            │ 1.0.3                  │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ API Version Supported  │ 1.0.3                  │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Driver Version         │ 361.0.0                │
08-16 21:22:21.008  5096  5111 I VulkanInfo: ├────────────────────────┼────────────────────────┤
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Image count            │ 2 - 3                  │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Array layers           │ 1                      │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Image size (current)   │ 1200x1920              │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Image size (extent)    │ 1x1 - 4096x4096        │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Usage flags            │ 9f                     │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Current transform      │ 04                     │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Allowed transforms     │ 10f                    │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ Surface formats        │ 3 of 185               │
08-16 21:22:21.008  5096  5111 I VulkanInfo: ├────────────────────────┼────────────────────────┤
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ INSTANCE EXTENSIONS    │                        │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ 1                      │ VK_KHR_surface         │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ 2                      │ VK_KHR_android_surface │
08-16 21:22:21.008  5096  5111 I VulkanInfo: ├────────────────────────┼────────────────────────┤
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ DEVICE EXTENSIONS      │                        │
08-16 21:22:21.008  5096  5111 I VulkanInfo: │ 1                      │ VK_KHR_swapchain       │
08-16 21:22:21.008  5096  5111 I VulkanInfo: ╰────────────────────────┴────────────────────────╯
```

If you enable some of validation layers, they'd get listed too.

## [VulkanDraw](https://github.com/vulkan-go/demos/vulkandraw)

A fully functional drawing example, ported from [googlesamples/android-vulkan-tutorials/tutorial05_triangle](https://github.com/googlesamples/android-vulkan-tutorials). 1KLOC, nothing special, I liked the way the original code has been organized. This was the first piece of some real code I wrote using the Vulkan API and it really delivered my the idea behind it. Anyway, I used wrong method of handling errors here, just to see how it would feel after I'm done. It feels horrible, must've used asserts like in the next demo. All the debug and validation layers are disabled by default

<a href="https://cl.ly/410g1n2r041E/screen.png"><img src="https://cl.ly/410g1n2r041E/screen.png" width="500"></a>

## [VulkanCube](https://github.com/vulkan-go/demos/vulkancube)

Well, after the first demo I felt like I'm ready for a big deal. A cube! It's a drawing example with dynamic state
that I ported from the Cube demo under [googlesamples/vulkan-basic-samples/LunarGSamples/Demos](https://github.com/googlesamples/vulkan-basic-samples/tree/master/LunarGSamples/Demos). 1.6KLOC (from 3KLOC), not a big deal (sarcasm). Surely that was a total suicide plus I figured out that the repo had an outdated version of this demo and in the LunarG mainstream a lots of things are fixed now. Also the code is very poorly organized.

But there are some positiva moments too. For example, I figured out how to use validation layers, so I could debug a few nasty typos and accidental bugs. Also I tried and satisfied by error handling method I chose here. Definitely would recommend to organize the error checking to be assert-like.

And anyways, that was a fun trip and actually this thing works: validation layers are quiet now, these thousands of lines do useful work and some parts can be reused as snippets. It just draws nothing that I can show you :)

I decided to fallaback from this example for a moment, maybe I'll do another cube demo from scratch when I'll get used to Vulkan better. Feel free to debug this thing. Validation layers and debug reporting are enabled in the code.

## Contibute yours

Do it! Just do it! 10KLOC is just a warm-up for you.

