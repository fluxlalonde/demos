VulkanInfo
===========

This is a simple Vulkan + Android Go app template.

### Initial setup

Refer to [github.com/xlab/android-go/example#prerequisites](https://github.com/xlab/android-go/tree/master/example#prerequisites) for the first run instructions for Android NDK. Please note that you'll need to obtain a device with Android N or Nvidia Shield K1, because currently only these provide the native Vulkan API support.

Once setup correctly, this course of actions would be sufficient:

```bash
make # you can debug any bugs in Go code here
make install
make listen
```

After you make changes, a simple `make && make install` would be sufficient to have your app updated.
See the [Makefile](/Makefile) for details on each step.

### Description

VulkanInfo gathers info about your Vulkan environment's properties and prints them to the log.
After you install and run the target app, check out the ADB logcat logs:

```
$ make listen
adb logcat -c
adb logcat *:S VulkanInfo
--------- beginning of system
--------- beginning of main
08-13 12:59:39.311 11932 11946 I VulkanInfo: ╭───────────────────────────────────────────╮
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ VULKAN PROPERTIES AND SURFACE CAPABILITES │
08-13 12:59:39.311 11932 11946 I VulkanInfo: ├────────────────────────┬──────────────────┤
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Physical Device Name   │ NVIDIA Tegra K1  │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Physical Device Vendor │ 10de             │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Physical Device Type   │ Integrated GPU   │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Physical GPUs          │ 1                │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ API Version            │ 1.0.3            │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ API Version Supported  │ 1.0.3            │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Driver Version         │ 361.0.0          │
08-13 12:59:39.311 11932 11946 I VulkanInfo: ├────────────────────────┼──────────────────┤
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Image count            │ 2 - 3            │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Array layers           │ 1                │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Image size (current)   │ 1200x1920        │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Image size (extent)    │ 1x1 - 4096x4096  │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Usage flags            │ 9f               │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Current transform      │ 04               │
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Allowed transforms     │ 10f              │
08-13 12:59:39.311 11932 11946 I VulkanInfo: ├────────────────────────┼──────────────────┤
08-13 12:59:39.311 11932 11946 I VulkanInfo: │ Surface formats        │ 3 of 185         │
08-13 12:59:39.311 11932 11946 I VulkanInfo: ╰────────────────────────┴──────────────────╯
```
