package main

import (
	"bytes"
	"image"
	"image/draw"
	"log"
	"unsafe"

	"github.com/lmittmann/ppm"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/android-go/android"
	"github.com/xlab/linmath"
)

// enableDebug is disabled by default since VK_EXT_debug_report
// is not guaranteed to be present on a device.
//
// Nvidia Shield K1 fw 1.3.0 lacks this extension,
// on fw 1.2.0 it simply doesn't work. Facepalm.
const enableDebug = true

const demoTextureCount = 1

// TextureObject tracks all objects related to a texture.
type TextureObject struct {
	sampler     vk.Sampler
	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	width  int
	height int
}

type Vertex struct {
	posX, posY, posZ, posW float32 // Position data
	r, g, b, a             float32 // Color
}

type VertexPosTex struct {
	posX, posY, posZ, posW float32 // Position data
	u, v, s, t             float32 // Texcoord
}

type SwapchainBuffersInfo struct {
	image vk.Image
	cmd   vk.CommandBuffer
	view  vk.ImageView
}

type DepthInfo struct {
	format vk.Format

	image    vk.Image
	memAlloc vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

type UniformInfo struct {
	buf      vk.Buffer
	memAlloc vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	bufInfo  vk.DescriptorBufferInfo
}

type Demo struct {
	surface          vk.Surface
	prepared         bool
	useStagingBuffer bool

	instance vk.Instance
	gpu      vk.PhysicalDevice
	device   vk.Device
	queue    vk.Queue

	gpuProps   vk.PhysicalDeviceProperties
	queueProps []vk.QueueFamilyProperties
	memProps   vk.PhysicalDeviceMemoryProperties

	enabledExtensionCount  uint32
	enabledLayerCount      uint32
	extensionNames         []string
	deviceValidationLayers []string

	width      uint32
	height     uint32
	format     vk.Format
	colorSpace vk.ColorSpace

	swapchainImageCount    int
	swapchain              vk.Swapchain
	graphicsQueueNodeIndex uint32
	buffers                []SwapchainBuffersInfo

	cmdPool  vk.CommandPool
	depth    DepthInfo
	uniform  UniformInfo
	textures []TextureObject

	cmd            vk.CommandBuffer // for initialization commands
	pipelineLayout vk.PipelineLayout
	descLayout     vk.DescriptorSetLayout
	pipelineCache  vk.PipelineCache
	renderPass     vk.RenderPass
	pipeline       vk.Pipeline

	vsName  string
	fsName  string
	texName string

	projectionMat *linmath.Mat4x4
	viewMat       *linmath.Mat4x4
	modelMat      *linmath.Mat4x4

	spinAngle     float32
	spinIncrement float32
	pause         bool

	descPool vk.DescriptorPool
	descSet  vk.DescriptorSet

	framebuffers []vk.Framebuffer
	quit         bool

	dbgCallback vk.DebugReportCallback

	currentBuffer uint32
	queueCount    int
}

func (d *Demo) flushInitCmd() {
	if d.cmd == nil {
		return
	}
	err := vk.EndCommandBuffer(d.cmd)
	orPanic(err)

	cmdBuffers := []vk.CommandBuffer{d.cmd}
	submitInfo := []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    cmdBuffers,
	}}
	err = vk.QueueSubmit(d.queue, 1, submitInfo, vk.NullHandle)
	orPanic(err)
	err = vk.QueueWaitIdle(d.queue)
	orPanic(err)

	vk.FreeCommandBuffers(d.device, d.cmdPool, 1, cmdBuffers)
	d.cmd = nil
}

func (d *Demo) setImageLayout(image vk.Image, aspectMask vk.ImageAspectFlags,
	oldLayout vk.ImageLayout, newLayout vk.ImageLayout, srcAccessMask vk.AccessFlagBits) {

	if d.cmd == nil {
		d.beginCmdBuffer()
	}
	imgMemoryBarrier := vk.ImageMemoryBarrier{
		SType:         vk.StructureTypeImageMemoryBarrier,
		SrcAccessMask: vk.AccessFlags(srcAccessMask),
		OldLayout:     oldLayout,
		NewLayout:     newLayout,
		Image:         image,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: aspectMask,
			LevelCount: 1,
			LayerCount: 1,
		},
	}
	switch newLayout {
	case vk.ImageLayoutTransferDstOptimal:
		// Make sure anything that was copying from this image has completed.
		imgMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferReadBit)
	case vk.ImageLayoutColorAttachmentOptimal:
		imgMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessColorAttachmentWriteBit)
	case vk.ImageLayoutDepthStencilAttachmentOptimal:
		imgMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessDepthStencilAttachmentWriteBit)
	case vk.ImageLayoutShaderReadOnlyOptimal:
		// Make sure any Copy or CPU writes to image are flushed.
		imgMemoryBarrier.DstAccessMask = vk.AccessFlags(
			vk.AccessShaderReadBit | vk.AccessInputAttachmentReadBit)
	}
	const srcStages = vk.PipelineStageFlags(vk.PipelineStageTopOfPipeBit)
	const dstStages = vk.PipelineStageFlags(vk.PipelineStageTopOfPipeBit)
	barriers := []vk.ImageMemoryBarrier{imgMemoryBarrier}
	vk.CmdPipelineBarrier(d.cmd, srcStages, dstStages, 0, 0, nil, 0, nil, 1, barriers)
}

func (d *Demo) beginCmdBuffer() {
	allocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        d.cmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}
	commandBuffers := make([]vk.CommandBuffer, 1)
	err := vk.AllocateCommandBuffers(d.device, &allocateInfo, commandBuffers)
	orPanic(err)
	d.cmd = commandBuffers[0]

	beginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		PInheritanceInfo: []vk.CommandBufferInheritanceInfo{{
			SType: vk.StructureTypeCommandBufferInheritanceInfo,
		}},
	}
	err = vk.BeginCommandBuffer(d.cmd, &beginInfo)
	orPanic(err)
}

func (d *Demo) drawBuildCmd(cmdBuf vk.CommandBuffer) {
	cmdBufferBeginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		PInheritanceInfo: []vk.CommandBufferInheritanceInfo{{
			SType: vk.StructureTypeCommandBufferInheritanceInfo,
		}},
	}
	clearValues := make([]vk.ClearValue, 2)
	clearValues[1].SetDepthStencil(1, 0)
	clearValues[0].SetColor([]float32{
		0.2, 0.2, 0.2, 0.2,
	})
	renderPassBeginInfo := vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  d.renderPass,
		Framebuffer: d.framebuffers[d.currentBuffer],
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{
				X: 0, Y: 0,
			},
			Extent: vk.Extent2D{
				Width:  d.width,
				Height: d.height,
			},
		},
		ClearValueCount: 2,
		PClearValues:    clearValues,
	}
	err := vk.BeginCommandBuffer(cmdBuf, &cmdBufferBeginInfo)
	orPanic(err)

	vk.CmdBeginRenderPass(cmdBuf, &renderPassBeginInfo, vk.SubpassContentsInline)
	vk.CmdBindPipeline(cmdBuf, vk.PipelineBindPointGraphics, d.pipeline)

	descriptorSets := []vk.DescriptorSet{
		d.descSet,
	}
	vk.CmdBindDescriptorSets(cmdBuf, vk.PipelineBindPointGraphics, d.pipelineLayout,
		0, 1, descriptorSets, 0, nil)

	viewports := []vk.Viewport{{
		MinDepth: 0.0,
		MaxDepth: 1.0,
		X:        0,
		Y:        0,
		Width:    float32(d.width),
		Height:   float32(d.height),
	}}
	vk.CmdSetViewport(cmdBuf, 0, 1, viewports)

	scissors := []vk.Rect2D{{
		Extent: vk.Extent2D{
			Width:  d.width,
			Height: d.height,
		},
		Offset: vk.Offset2D{
			X: 0, Y: 0,
		},
	}}
	vk.CmdSetScissor(cmdBuf, 0, 1, scissors)

	vk.CmdDraw(cmdBuf, 12*3, 1, 0, 0)
	vk.CmdEndRenderPass(cmdBuf)

	prePresentBarrier := vk.ImageMemoryBarrier{
		SType:               vk.StructureTypeImageMemoryBarrier,
		SrcAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
		DstAccessMask:       vk.AccessFlags(vk.AccessMemoryReadBit),
		OldLayout:           vk.ImageLayoutColorAttachmentOptimal,
		NewLayout:           vk.ImageLayoutPresentSrc,
		SrcQueueFamilyIndex: vk.QueueFamilyIgnored,
		DstQueueFamilyIndex: vk.QueueFamilyIgnored,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
			LevelCount: 1,
			LayerCount: 1,
		},
	}
	prePresentBarrier.Image = d.buffers[d.currentBuffer].image
	const srcStages = vk.PipelineStageFlags(vk.PipelineStageAllCommandsBit)
	const dstStages = vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit)
	barriers := []vk.ImageMemoryBarrier{prePresentBarrier}
	vk.CmdPipelineBarrier(cmdBuf, srcStages, dstStages, 0, 0, nil, 0, nil, 1, barriers)

	err = vk.EndCommandBuffer(cmdBuf)
	orPanic(err)
}

func (d *Demo) updateDataBuffer() {
	var (
		MVP   = new(linmath.Mat4x4)
		Model = new(linmath.Mat4x4)
		VP    = new(linmath.Mat4x4)
	)
	VP.Mult(d.projectionMat, d.viewMat)
	// Rotate 22.5 degrees around the Y axis
	Model.Dup(d.modelMat)
	angle := linmath.DegreesToRadians(d.spinAngle)
	d.modelMat.Rotate(Model, 0, 1, 0, angle)
	MVP.Mult(VP, d.modelMat)

	var data unsafe.Pointer
	err := vk.MapMemory(d.device, d.uniform.mem, 0, d.uniform.memAlloc.AllocationSize, 0, &data)
	orPanic(err)

	vertexData := MVP.Slice()
	n := vk.MemCopyFloat32(data, vertexData)
	if n != len(vertexData) {
		log.Println("[WARN] failed to copy vertex data")
	}
	vk.UnmapMemory(d.device, d.uniform.mem)
}

func (d *Demo) draw() {
	var presentCompleteSemaphore vk.Semaphore
	semaphoreCreateInfo := vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	err := vk.CreateSemaphore(d.device, &semaphoreCreateInfo, nil, &presentCompleteSemaphore)
	orPanic(err)
	defer vk.DestroySemaphore(d.device, presentCompleteSemaphore, nil)

	err = vk.AcquireNextImage(d.device, d.swapchain, vk.MaxUint64,
		presentCompleteSemaphore, vk.NullHandle, &d.currentBuffer)
	switch err {
	case vk.ErrorOutOfDate:
		// d.swapchain is out of date (e.g. the window was resized) and
		// must be recreated:
		d.resize()
		d.draw()
		return
	case vk.Suboptimal:
		// d.swapchain is not as optimal as it could be, but the platform's
		// presentation engine will still present the image correctly.
	default:
		orPanic(err)
	}
	// Assume the command buffer has been run on current_buffer before so
	// we need to set the image layout back to ColorAttachmentOptimal
	d.setImageLayout(d.buffers[d.currentBuffer].image,
		vk.ImageAspectFlags(vk.ImageAspectColorBit), vk.ImageLayoutPresentSrc,
		vk.ImageLayoutColorAttachmentOptimal, 0)
	d.flushInitCmd()

	// Wait for the present complete semaphore to be signaled to ensure
	// that the image won't be rendered to until the presentation
	// engine has fully released ownership to the application, and it is
	// okay to render to the image.

	// FIXME/TODO: DEAL WITH VK_IMAGE_LAYOUT_PRESENT_SRC_KHR
	submitInfos := []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		WaitSemaphoreCount: 1,
		PWaitSemaphores: []vk.Semaphore{
			presentCompleteSemaphore,
		},
		PWaitDstStageMask: []vk.PipelineStageFlags{
			vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit),
		},
		CommandBufferCount: 1,
		PCommandBuffers: []vk.CommandBuffer{
			d.buffers[d.currentBuffer].cmd,
		},
	}}
	err = vk.QueueSubmit(d.queue, 1, submitInfos, vk.NullHandle)
	orPanic(err)

	presentInfo := vk.PresentInfo{
		SType:          vk.StructureTypePresentInfo,
		SwapchainCount: 1,
		PSwapchains: []vk.Swapchain{
			d.swapchain,
		},
		PImageIndices: []uint32{
			d.currentBuffer,
		},
	}
	err = vk.QueuePresent(d.queue, &presentInfo)
	switch err {
	case vk.ErrorOutOfDate:
		// d.swapchain is out of date (e.g. the window was resized) and
		// must be recreated:
		d.resize()
	case vk.Suboptimal:
		// d.swapchain is not as optimal as it could be, but the platform's
		// presentation engine will still present the image correctly.
	default:
		orPanic(err)
	}

	err = vk.QueueWaitIdle(d.queue)
	orPanic(err)
}

func (d *Demo) prepareSwapchain() {
	vk.GetPhysicalDeviceProperties(d.gpu, &d.gpuProps)
	var queueCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(d.gpu, &queueCount, nil)
	d.queueCount = int(queueCount)
	orPanicWith(d.queueCount >= 1, "Physical device has no avalable queues")
	d.queueProps = make([]vk.QueueFamilyProperties, d.queueCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(d.gpu, &queueCount, d.queueProps)

	gfxQueueIdx := -1
	for i := 0; i < d.queueCount; i++ {
		props := d.queueProps[i]
		props.Deref()
		if props.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			gfxQueueIdx = i
			break
		}
	}
	orPanicWith(gfxQueueIdx >= 0, "Cannot find queue with graphics support")

	var surfCapabilities vk.SurfaceCapabilities
	err := vk.GetPhysicalDeviceSurfaceCapabilities(d.gpu, d.surface, &surfCapabilities)
	orPanic(err)

	var presentModeCount uint32
	err = vk.GetPhysicalDeviceSurfacePresentModes(d.gpu, d.surface, &presentModeCount, nil)
	orPanic(err)
	presentModes := make([]vk.PresentMode, presentModeCount)
	err = vk.GetPhysicalDeviceSurfacePresentModes(d.gpu, d.surface, &presentModeCount, presentModes)
	orPanic(err)

	var swapchainExtent vk.Extent2D
	surfCapabilities.Deref()
	currentExtent := surfCapabilities.CurrentExtent
	currentExtent.Deref()
	// width and height are either both undefined, or both set.
	if currentExtent.Width == vk.MaxUint32 {
		// If the surface size is undefined, the size is set to
		// the size of the images requested.
		swapchainExtent.Width = d.width
		swapchainExtent.Height = d.height
	} else {
		// If the surface size is defined, the swap chain size must match
		swapchainExtent = currentExtent
		d.width = currentExtent.Width
		d.height = currentExtent.Height
	}

	supportsPresent := make([]vk.Bool32, d.queueCount)
	for i := 0; i < d.queueCount; i++ {
		vk.GetPhysicalDeviceSurfaceSupport(d.gpu, uint32(i), d.surface, &supportsPresent[i])
	}

	var graphicsQueueNodeIndex = vk.MaxUint32
	var presentQueueNodeIndex = vk.MaxUint32
	for i := 0; i < d.queueCount; i++ {
		props := d.queueProps[i]
		props.Deref()
		if props.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			if graphicsQueueNodeIndex == vk.MaxUint32 {
				graphicsQueueNodeIndex = uint32(i)
			}
			if supportsPresent[i] == vk.Bool32(vk.True) {
				graphicsQueueNodeIndex = uint32(i)
				presentQueueNodeIndex = uint32(i)
				break
			}
		}
	}
	if presentQueueNodeIndex == vk.MaxUint32 {
		// If didn't find a queue that supports both graphics and present, then
		// find a separate present queue.
		for i := 0; i < d.queueCount; i++ {
			if supportsPresent[i] == vk.Bool32(vk.True) {
				presentQueueNodeIndex = uint32(i)
				break
			}
		}
	}
	orPanicWith(graphicsQueueNodeIndex != vk.MaxUint32 &&
		presentQueueNodeIndex != vk.MaxUint32, "Could not find a graphics or present queues")

	// TODO: Add support for separate queues, including presentation,
	//       synchronization, and appropriate tracking for QueueSubmit.
	// NOTE: While it is possible for an application to use a separate graphics
	//       and a present queues, this demo program assumes it is only using
	//       one:
	orPanicWith(graphicsQueueNodeIndex == presentQueueNodeIndex,
		"Could not find a common graphics and present queues")
	d.graphicsQueueNodeIndex = graphicsQueueNodeIndex
	var queue vk.Queue
	vk.GetDeviceQueue(d.device, d.graphicsQueueNodeIndex, 0, &queue)
	d.queue = queue

	// If mailbox mode is available, use it, as is the lowest-latency non-
	// tearing mode. If not, try IMMEDIATE which will usually be available,
	// and is fastest (though it tears).  If not, fall back to FIFO which is
	// always available.
	swapchainPresentMode := vk.PresentModeFifo
	for i := uint32(0); i < presentModeCount; i++ {
		if presentModes[i] == vk.PresentModeMailbox {
			swapchainPresentMode = vk.PresentModeMailbox
			break
		}
		if swapchainPresentMode != vk.PresentModeMailbox &&
			presentModes[i] == vk.PresentModeImmediate {
			swapchainPresentMode = vk.PresentModeImmediate
		}
	}
	// Determine the number of VkImage's to use in the swap chain (we desire to
	// own only 1 image at a time, besides the images being displayed and
	// queued for display):
	desiredNumberOfSwapchainImages := surfCapabilities.MinImageCount + 1
	if surfCapabilities.MaxImageCount > 0 &&
		desiredNumberOfSwapchainImages > surfCapabilities.MaxImageCount {
		// Application must settle for fewer images than desired:
		desiredNumberOfSwapchainImages = surfCapabilities.MaxImageCount
	}
	oldSwapchain := d.swapchain
	queueFamily := []uint32{0}
	swapchainCreateInfo := vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         d.surface,
		MinImageCount:   surfCapabilities.MinImageCount,
		ImageFormat:     d.format,
		ImageColorSpace: d.colorSpace,
		ImageExtent:     surfCapabilities.CurrentExtent,
		ImageUsage:      vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		PreTransform:    vk.SurfaceTransformIdentityBit,
		CompositeAlpha:  vk.CompositeAlphaInheritBit,

		ImageArrayLayers:      1,
		QueueFamilyIndexCount: 1,
		PQueueFamilyIndices:   queueFamily,
		ImageSharingMode:      vk.SharingModeExclusive,
		PresentMode:           swapchainPresentMode,
		OldSwapchain:          oldSwapchain,
		Clipped:               vk.True,
	}
	err = vk.CreateSwapchain(d.device, &swapchainCreateInfo, nil, &d.swapchain)
	orPanic(err)

	// If we just re-created an existing swapchain, we should destroy the old
	// swapchain at this point.
	// Note: destroying the swapchain also cleans up all its associated
	// presentable images once the platform is done with them.
	if oldSwapchain != vk.NullHandle {
		vk.DestroySwapchain(d.device, oldSwapchain, nil)
	}

	var imgCount uint32
	err = vk.GetSwapchainImages(d.device, d.swapchain, &imgCount, nil)
	orPanic(err)
	d.swapchainImageCount = int(imgCount)
	swapchainImages := make([]vk.Image, d.swapchainImageCount)
	err = vk.GetSwapchainImages(d.device, d.swapchain, &imgCount, swapchainImages)
	orPanic(err)

	d.buffers = make([]SwapchainBuffersInfo, d.swapchainImageCount)
	for i := range d.buffers {
		viewCreateInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			ViewType: vk.ImageViewType2d,
			Format:   d.format,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			},
		}
		d.buffers[i].image = swapchainImages[i]
		// Render loop will expect image to have been used before and in
		// vk.ImageLayoutPresentSrc
		// layout and will change to ColorAttachmentOptimal, so init the image
		// to that state
		d.setImageLayout(d.buffers[i].image, vk.ImageAspectFlags(vk.ImageAspectColorBit),
			vk.ImageLayoutUndefined, vk.ImageLayoutPresentSrc, 0)

		viewCreateInfo.Image = d.buffers[i].image
		err = vk.CreateImageView(d.device, &viewCreateInfo, nil, &d.buffers[i].view)
		orPanic(err)
	}
}

func (d *Demo) prepareDepth() {
	const depthFormat = vk.FormatD16Unorm
	d.depth.format = depthFormat

	imageInfo := vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    depthFormat,
		Extent: vk.Extent3D{
			Width:  d.width,
			Height: d.height,
			Depth:  1,
		},
		MipLevels:   1,
		ArrayLayers: 1,
		Samples:     vk.SampleCount1Bit,
		Tiling:      vk.ImageTilingOptimal,
		Usage:       vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
	}
	err := vk.CreateImage(d.device, &imageInfo, nil, &d.depth.image)
	orPanic(err)

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(d.device, d.depth.image, &memReqs)
	memReqs.Deref()
	d.depth.memAlloc = vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: 0, // see below
	}
	// FindMemoryTypeIndex with no memory requirements
	memTypeIdx, ok := vk.FindMemoryTypeIndex(d.gpu, memReqs.MemoryTypeBits, 0)
	orPanicWith(ok, "FindMemoryTypeIndex failed")
	d.depth.memAlloc.MemoryTypeIndex = memTypeIdx

	err = vk.AllocateMemory(d.device, &d.depth.memAlloc, nil, &d.depth.mem)
	orPanic(err)
	err = vk.BindImageMemory(d.device, d.depth.image, d.depth.mem, 0)
	orPanic(err)

	d.setImageLayout(d.depth.image, vk.ImageAspectFlags(vk.ImageAspectDepthBit),
		vk.ImageLayoutUndefined, vk.ImageLayoutDepthStencilAttachmentOptimal, 0)

	viewInfo := vk.ImageViewCreateInfo{
		SType:  vk.StructureTypeImageViewCreateInfo,
		Format: depthFormat,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectDepthBit),
			LevelCount: 1,
			LayerCount: 1,
		},
		ViewType: vk.ImageViewType2d,
	}
	viewInfo.Image = d.depth.image
	err = vk.CreateImageView(d.device, &viewInfo, nil, &d.depth.view)
	orPanic(err)
}

func loadTextureSize(name string) (w int, h int, err error) {
	data := MustAsset(name)
	r := bytes.NewReader(data)
	ppmCfg, err := ppm.DecodeConfig(r)
	if err != nil {
		return 0, 0, err
	}
	return ppmCfg.Width, ppmCfg.Height, nil
}

func loadTextureData(name string, layout vk.SubresourceLayout) ([]byte, error) {
	data := MustAsset(name)
	r := bytes.NewReader(data)
	img, err := ppm.Decode(r)
	if err != nil {
		return nil, err
	}
	newImg := image.NewRGBA(img.Bounds())
	newImg.Stride = int(layout.RowPitch)
	draw.Draw(newImg, newImg.Bounds(), img, image.ZP, draw.Src)
	return []byte(newImg.Pix), nil
}

func (d *Demo) prepareTextureImage(name string, tiling vk.ImageTiling,
	usageFlags vk.ImageUsageFlags, memProps vk.MemoryPropertyFlagBits) TextureObject {

	const texFormat = vk.FormatR8g8b8a8Unorm
	w, h, texErr := loadTextureSize(name)
	orPanic(texErr)

	texObj := TextureObject{
		width: w, height: h,
	}
	log.Printf("[INFO] loaded texture %s with dimensions %dx%d", name, w, h)

	imgCreateInfo := vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    texFormat,
		Extent: vk.Extent3D{
			Width:  uint32(w),
			Height: uint32(h),
			Depth:  1,
		},
		MipLevels:     1,
		ArrayLayers:   1,
		Samples:       vk.SampleCount1Bit,
		Tiling:        tiling,
		Usage:         usageFlags,
		InitialLayout: vk.ImageLayoutPreinitialized,
	}

	err := vk.CreateImage(d.device, &imgCreateInfo, nil, &texObj.image)
	orPanic(err)
	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(d.device, texObj.image, &memReqs)
	memReqs.Deref()
	texObj.memAlloc = vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: 0, // see below
	}
	memTypeIdx, ok := vk.FindMemoryTypeIndex(d.gpu, memReqs.MemoryTypeBits, memProps)
	orPanicWith(ok, "FindMemoryTypeIndex failed")
	texObj.memAlloc.MemoryTypeIndex = memTypeIdx

	err = vk.AllocateMemory(d.device, &texObj.memAlloc, nil, &texObj.mem)
	orPanic(err)
	err = vk.BindImageMemory(d.device, texObj.image, texObj.mem, 0)
	orPanic(err)

	memHostVisible := memProps&vk.MemoryPropertyHostVisibleBit != 0
	if memHostVisible {
		subResource := vk.ImageSubresource{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
		}
		var layout vk.SubresourceLayout
		vk.GetImageSubresourceLayout(d.device, texObj.image, &subResource, &layout)
		layout.Deref()

		rgbaData, texErr := loadTextureData(name, layout)
		orPanic(texErr)

		var data unsafe.Pointer
		err := vk.MapMemory(d.device, texObj.mem, 0, texObj.memAlloc.AllocationSize, 0, &data)
		orPanic(err)

		// TODO(xlab): this data could be read directly
		n := vk.MemCopyByte(data, rgbaData)
		if n != len(rgbaData) {
			log.Printf("[WARN] failed to load texture %dx%d from %s",
				texObj.width, texObj.height, name)
		}

		vk.UnmapMemory(d.device, texObj.mem)
	}

	texObj.imageLayout = vk.ImageLayoutShaderReadOnlyOptimal
	// setting the image layout does not reference the actual memory so no need
	// to add a mem ref.
	d.setImageLayout(texObj.image, vk.ImageAspectFlags(vk.ImageAspectColorBit),
		vk.ImageLayoutPreinitialized, texObj.imageLayout, vk.AccessHostWriteBit)

	return texObj
}

func (d *Demo) destroyTextureImage(obj TextureObject) {
	vk.FreeMemory(d.device, obj.mem, nil)
	vk.DestroyImage(d.device, obj.image, nil)
}

func (d *Demo) prepareTextures(names ...string) {
	const texFormat = vk.FormatR8g8b8a8Unorm

	var props vk.FormatProperties
	vk.GetPhysicalDeviceFormatProperties(d.gpu, texFormat, &props)
	props.Deref()

	prepareTexture := func(i int, texName string) {
		switch {
		case (props.LinearTilingFeatures&
			vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0) && !d.useStagingBuffer:
			// Device can texture using linear textures.
			d.textures[i] = d.prepareTextureImage(texName, vk.ImageTilingLinear,
				vk.ImageUsageFlags(vk.ImageUsageSampledBit), vk.MemoryPropertyHostVisibleBit)

		case props.OptimalTilingFeatures&
			vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0:
			// Must use staging buffer to copy linear texture to optimized.
			var staging TextureObject
			staging = d.prepareTextureImage(texName, vk.ImageTilingLinear,
				vk.ImageUsageFlags(vk.ImageUsageTransferSrcBit), vk.MemoryPropertyHostVisibleBit)

			d.textures[i] = d.prepareTextureImage(texName, vk.ImageTilingOptimal,
				vk.ImageUsageFlags(vk.ImageUsageTransferDstBit|vk.ImageUsageSampledBit),
				vk.MemoryPropertyDeviceLocalBit)

			d.setImageLayout(staging.image, vk.ImageAspectFlags(vk.ImageAspectColorBit),
				staging.imageLayout, vk.ImageLayoutTransferSrcOptimal, 0)
			d.setImageLayout(d.textures[i].image, vk.ImageAspectFlags(vk.ImageAspectColorBit),
				d.textures[i].imageLayout, vk.ImageLayoutTransferSrcOptimal, 0)

			copyRegions := []vk.ImageCopy{{
				SrcSubresource: vk.ImageSubresourceLayers{
					AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
					LayerCount: 1,
				},
				DstSubresource: vk.ImageSubresourceLayers{
					AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
					LayerCount: 1,
				},
				Extent: vk.Extent3D{
					Width:  uint32(staging.width),
					Height: uint32(staging.height),
					Depth:  1,
				},
			}}
			const srcImageLayout = vk.ImageLayoutTransferSrcOptimal
			const dstImageLayout = vk.ImageLayoutTransferDstOptimal
			vk.CmdCopyImage(d.cmd, staging.image, srcImageLayout,
				d.textures[i].image, dstImageLayout, 1, copyRegions)

			d.setImageLayout(d.textures[i].image, vk.ImageAspectFlags(vk.ImageAspectColorBit),
				vk.ImageLayoutTransferDstOptimal, d.textures[i].imageLayout, 0)
			d.flushInitCmd()
			d.destroyTextureImage(staging)
		default:
			orPanicWith(false, "No support for R8G8B8A8_UNORM as texture image format.")
		}

		samplerInfo := vk.SamplerCreateInfo{
			SType:                   vk.StructureTypeSamplerCreateInfo,
			MagFilter:               vk.FilterNearest,
			MinFilter:               vk.FilterNearest,
			MipmapMode:              vk.SamplerMipmapModeNearest,
			AddressModeU:            vk.KhrSamplerMirrorClampToEdge,
			AddressModeV:            vk.KhrSamplerMirrorClampToEdge,
			AddressModeW:            vk.KhrSamplerMirrorClampToEdge,
			AnisotropyEnable:        vk.False,
			MaxAnisotropy:           1,
			CompareOp:               vk.CompareOpNever,
			BorderColor:             vk.BorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: vk.False,
		}
		imageViewInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			ViewType: vk.ImageViewType2d,
			Format:   texFormat,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LayerCount: 1,
				LevelCount: 1,
			},
		}
		err := vk.CreateSampler(d.device, &samplerInfo, nil, &d.textures[i].sampler)
		orPanic(err)

		imageViewInfo.Image = d.textures[i].image
		err = vk.CreateImageView(d.device, &imageViewInfo, nil, &d.textures[i].view)
		orPanic(err)
	}

	// execute on all submitted texture names
	for i, texName := range names {
		prepareTexture(i, texName)
	}
}

func (d *Demo) prepareCubeDataBuffer() {
	var (
		MVP = new(linmath.Mat4x4)
		VP  = new(linmath.Mat4x4)
	)
	VP.Mult(d.projectionMat, d.viewMat)
	MVP.Mult(VP, d.modelMat)
	log.Println(linmath.DumpMatrix(MVP, "MVP"))

	var bufData vkTexCubeUniform
	MVP.CopyTo(&bufData.mvp)
	for i := 0; i < 12*3; i++ {
		bufData.position[i][0] = g_vertex_buffer_data[i*3]
		bufData.position[i][1] = g_vertex_buffer_data[i*3+1]
		bufData.position[i][2] = g_vertex_buffer_data[i*3+2]
		bufData.position[i][3] = 1
		bufData.attr[i][0] = g_uv_buffer_data[2*i]
		bufData.attr[i][1] = g_uv_buffer_data[2*i+1]
		bufData.attr[i][2] = 0
		bufData.attr[i][3] = 0
	}

	bufInfo := vk.BufferCreateInfo{
		SType: vk.StructureTypeBufferCreateInfo,
		Usage: vk.BufferUsageFlags(vk.BufferUsageUniformBufferBit),
		Size:  vk.DeviceSize(bufData.Sizeof()),
	}
	err := vk.CreateBuffer(d.device, &bufInfo, nil, &d.uniform.buf)
	orPanic(err)

	var memReqs vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(d.device, d.uniform.buf, &memReqs)
	memReqs.Deref()

	d.uniform.memAlloc = vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: 0, // see below
	}
	memTypeIdx, ok := vk.FindMemoryTypeIndex(d.gpu, memReqs.MemoryTypeBits,
		vk.MemoryPropertyHostVisibleBit)
	orPanicWith(ok, "FindMemoryTypeIndex failed")
	d.uniform.memAlloc.MemoryTypeIndex = memTypeIdx

	err = vk.AllocateMemory(d.device, &d.uniform.memAlloc, nil, &d.uniform.mem)
	orPanic(err)
	var data unsafe.Pointer
	err = vk.MapMemory(d.device, d.uniform.mem, 0, d.uniform.memAlloc.AllocationSize, 0, &data)
	orPanic(err)

	toCopy := bufData.Slice()
	n := vk.MemCopyFloat32(data, toCopy)
	if n != len(toCopy) {
		log.Println("[WARN] failed to copy uniform data")
	}
	vk.UnmapMemory(d.device, d.uniform.mem)

	err = vk.BindBufferMemory(d.device, d.uniform.buf, d.uniform.mem, 0)
	orPanic(err)

	d.uniform.bufInfo.Free()
	d.uniform.bufInfo = vk.DescriptorBufferInfo{
		Buffer: d.uniform.buf,
		Offset: 0,
		Range:  vk.DeviceSize(bufData.Sizeof()),
	}
}
func (d *Demo) prepareDescriptorLayout() {
	layoutBindings := []vk.DescriptorSetLayoutBinding{{
		Binding:         0,
		DescriptorType:  vk.DescriptorTypeUniformBuffer,
		DescriptorCount: 1,
		StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit),
	}, {
		Binding:         1,
		DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
		DescriptorCount: demoTextureCount,
		StageFlags:      vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
	}}
	descLayoutInfo := vk.DescriptorSetLayoutCreateInfo{
		SType:        vk.StructureTypeDescriptorSetLayoutCreateInfo,
		BindingCount: 2,
		PBindings:    layoutBindings,
	}
	err := vk.CreateDescriptorSetLayout(d.device, &descLayoutInfo, nil, &d.descLayout)
	orPanic(err)

	layouts := []vk.DescriptorSetLayout{
		d.descLayout,
	}
	pipelineLayoutCreateInfo := vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: 1,
		PSetLayouts:    layouts,
	}
	err = vk.CreatePipelineLayout(d.device, &pipelineLayoutCreateInfo, nil, &d.pipelineLayout)
	orPanic(err)
}

func (d *Demo) prepareRenderPass() {
	attachments := []vk.AttachmentDescription{{
		Format:         d.format,
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpStore,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		InitialLayout:  vk.ImageLayoutColorAttachmentOptimal,
		FinalLayout:    vk.ImageLayoutColorAttachmentOptimal,
	}, {
		Format:         d.depth.format,
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpDontCare,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		InitialLayout:  vk.ImageLayoutDepthStencilAttachmentOptimal,
		FinalLayout:    vk.ImageLayoutDepthStencilAttachmentOptimal,
	}}
	colorReferences := []vk.AttachmentReference{{
		Attachment: 0, Layout: vk.ImageLayoutColorAttachmentOptimal,
	}}
	depthReferences := []vk.AttachmentReference{{
		Attachment: 1, Layout: vk.ImageLayoutDepthStencilAttachmentOptimal,
	}}
	subpasses := []vk.SubpassDescription{{
		PipelineBindPoint:       vk.PipelineBindPointGraphics,
		ColorAttachmentCount:    1,
		PColorAttachments:       colorReferences,
		PDepthStencilAttachment: depthReferences,
	}}
	renderPassInfo := vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: 2,
		PAttachments:    attachments,
		SubpassCount:    1,
		PSubpasses:      subpasses,
	}
	err := vk.CreateRenderPass(d.device, &renderPassInfo, nil, &d.renderPass)
	orPanic(err)
}

func loadShader(device vk.Device, name string) vk.ShaderModule {
	spvCode := MustAsset(name)
	shaderModuleInfo := vk.ShaderModuleCreateInfo{
		SType:    vk.StructureTypeShaderModuleCreateInfo,
		CodeSize: uint(len(spvCode)),
		PCode:    repackUint32(spvCode),
	}
	var module vk.ShaderModule
	err := vk.CreateShaderModule(device, &shaderModuleInfo, nil, &module)
	orPanic(err)
	return module
}

func (d *Demo) preparePipeline(vsName, fsName string) {
	vertexInputStateInfo := vk.PipelineVertexInputStateCreateInfo{
		SType: vk.StructureTypePipelineVertexInputStateCreateInfo,
	}
	inputAssemblyStateInfo := vk.PipelineInputAssemblyStateCreateInfo{
		SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology: vk.PrimitiveTopologyTriangleList,
	}
	pipelineRasterizationStateInfo := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeFill,
		CullMode:                vk.CullModeFlags(vk.CullModeNone),
		FrontFace:               vk.FrontFaceClockwise,
		DepthBiasEnable:         vk.False,
		LineWidth:               1,
	}
	pipelineColorBlendStateInfo := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		AttachmentCount: 1,
		PAttachments: []vk.PipelineColorBlendAttachmentState{{
			ColorWriteMask: 0xf, // RGBA
			BlendEnable:    vk.False,
		}},
	}
	dynamicStateEnables := []vk.DynamicState{
		vk.DynamicStateViewport,
		vk.DynamicStateScissor,
	}
	dynamicState := vk.PipelineDynamicStateCreateInfo{
		SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
		DynamicStateCount: 2,
		PDynamicStates:    dynamicStateEnables,
	}
	pipelineViewportStateInfo := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		ScissorCount:  1,
	}
	pipelineDepthStencilStateInfo := vk.PipelineDepthStencilStateCreateInfo{
		SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
		DepthTestEnable:       vk.True,
		DepthWriteEnable:      vk.True,
		DepthCompareOp:        vk.CompareOpLessOrEqual,
		DepthBoundsTestEnable: vk.False,
		Back: vk.StencilOpState{
			FailOp:    vk.StencilOpKeep,
			PassOp:    vk.StencilOpKeep,
			CompareOp: vk.CompareOpAlways,
		},
		Front: vk.StencilOpState{
			FailOp:    vk.StencilOpKeep,
			PassOp:    vk.StencilOpKeep,
			CompareOp: vk.CompareOpAlways,
		},
	}
	pipelineMultisampleStateInfo := vk.PipelineMultisampleStateCreateInfo{
		SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
		RasterizationSamples: vk.SampleCount1Bit,
	}

	vertexShader := loadShader(d.device, vsName)
	defer vk.DestroyShaderModule(d.device, vertexShader, nil)
	fragmentShader := loadShader(d.device, fsName)
	defer vk.DestroyShaderModule(d.device, fragmentShader, nil)

	shaderStages := []vk.PipelineShaderStageCreateInfo{
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vertexShader,
			PName:  "main\x00",
		},
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fragmentShader,
			PName:  "main\x00",
		},
	}
	pipelineInfos := []vk.GraphicsPipelineCreateInfo{{
		SType:               vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:              d.pipelineLayout,
		StageCount:          2, // vert + frag
		PStages:             shaderStages,
		PVertexInputState:   &vertexInputStateInfo,
		PInputAssemblyState: &inputAssemblyStateInfo,
		PRasterizationState: &pipelineRasterizationStateInfo,
		PColorBlendState:    &pipelineColorBlendStateInfo,
		PMultisampleState:   &pipelineMultisampleStateInfo,
		PViewportState:      &pipelineViewportStateInfo,
		PDepthStencilState:  &pipelineDepthStencilStateInfo,
		RenderPass:          d.renderPass,
		PDynamicState:       &dynamicState,
	}}

	pipelineCacheInfo := vk.PipelineCacheCreateInfo{
		SType: vk.StructureTypePipelineCacheCreateInfo,
	}
	err := vk.CreatePipelineCache(d.device, &pipelineCacheInfo, nil, &d.pipelineCache)
	orPanic(err)

	pipelines := make([]vk.Pipeline, 1)
	err = vk.CreateGraphicsPipelines(d.device, d.pipelineCache, 1, pipelineInfos, nil, pipelines)
	orPanic(err)
	d.pipeline = pipelines[0]
}

func (d *Demo) prepareDescriptorPool() {
	descriptorPoolInfo := vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		MaxSets:       1,
		PoolSizeCount: 2,
		PPoolSizes: []vk.DescriptorPoolSize{{
			Type:            vk.DescriptorTypeUniformBuffer,
			DescriptorCount: 1,
		}, {
			Type:            vk.DescriptorTypeCombinedImageSampler,
			DescriptorCount: demoTextureCount,
		}},
	}
	err := vk.CreateDescriptorPool(d.device, &descriptorPoolInfo, nil, &d.descPool)
	orPanic(err)
}

func (d *Demo) prepareDescriptorSet() {
	setLayouts := []vk.DescriptorSetLayout{
		d.descLayout,
	}
	descriptorSetAllocateInfo := vk.DescriptorSetAllocateInfo{
		SType:              vk.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     d.descPool,
		DescriptorSetCount: 1,
		PSetLayouts:        setLayouts,
	}
	err := vk.AllocateDescriptorSets(d.device, &descriptorSetAllocateInfo, &d.descSet)
	orPanic(err)

	texDescriptors := []vk.DescriptorImageInfo{{
		Sampler:     d.textures[0].sampler,
		ImageView:   d.textures[0].view,
		ImageLayout: vk.ImageLayoutGeneral,
	}}
	bufferInfo := []vk.DescriptorBufferInfo{
		d.uniform.bufInfo,
	}
	descriptorWrites := []vk.WriteDescriptorSet{{
		SType:           vk.StructureTypeWriteDescriptorSet,
		DstSet:          d.descSet,
		DescriptorCount: 1,
		DescriptorType:  vk.DescriptorTypeUniformBuffer,
		PBufferInfo:     bufferInfo,
	}, {
		SType:           vk.StructureTypeWriteDescriptorSet,
		DstSet:          d.descSet,
		DstBinding:      1,
		DescriptorCount: 1,
		DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
		PImageInfo:      texDescriptors,
	}}
	vk.UpdateDescriptorSets(d.device, 2, descriptorWrites, 0, nil)
}

func (d *Demo) prepareFramebuffers() {
	framebufferCreateInfo := vk.FramebufferCreateInfo{
		SType:           vk.StructureTypeFramebufferCreateInfo,
		RenderPass:      d.renderPass,
		AttachmentCount: 2,
		PAttachments: []vk.ImageView{
			vk.NullHandle, d.depth.view,
		},
		Width:  d.width,
		Height: d.height,
	}
	d.framebuffers = make([]vk.Framebuffer, d.swapchainImageCount)
	for i := range d.framebuffers {
		framebufferCreateInfo.PAttachments[0] = d.buffers[i].view
		err := vk.CreateFramebuffer(d.device, &framebufferCreateInfo, nil, &d.framebuffers[i])
		orPanic(err)
	}
}

func (d *Demo) Prepare(vsName, fsName, texName string) {
	d.prepareSurfaceCapabilities()
	vk.GetPhysicalDeviceMemoryProperties(d.gpu, &d.memProps)

	cmdPoolInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: d.graphicsQueueNodeIndex,
	}
	err := vk.CreateCommandPool(d.device, &cmdPoolInfo, nil, &d.cmdPool)
	orPanic(err)

	d.vsName = vsName
	d.fsName = fsName
	d.texName = texName
	d.textures = make([]TextureObject, 1)

	d.prepareSwapchain()
	d.prepareDepth()
	d.prepareTextures(texName)
	d.prepareCubeDataBuffer()

	d.prepareDescriptorLayout()
	d.prepareRenderPass()
	d.preparePipeline(vsName, fsName)

	cmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        d.cmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}
	buffers := make([]vk.CommandBuffer, 1)
	for i := 0; i < d.swapchainImageCount; i++ {
		err := vk.AllocateCommandBuffers(d.device, &cmdBufferAllocateInfo, buffers)
		orPanic(err)
		d.buffers[i].cmd = buffers[0]
	}

	d.prepareDescriptorPool()
	d.prepareDescriptorSet()
	d.prepareFramebuffers()

	for i := 0; i < d.swapchainImageCount; i++ {
		d.currentBuffer = uint32(i)
		d.drawBuildCmd(d.buffers[i].cmd)
	}

	// Prepare functions above may generate pipeline commands
	// that need to be flushed before beginning the render loop.
	d.flushInitCmd()
	d.currentBuffer = 0
	d.prepared = true
}

func (d *Demo) Cleanup() {
	d.prepared = false
	for i := 0; i < d.swapchainImageCount; i++ {
		vk.DestroyFramebuffer(d.device, d.framebuffers[i], nil)
	}
	d.framebuffers = nil

	vk.DestroyDescriptorPool(d.device, d.descPool, nil)
	vk.DestroyPipeline(d.device, d.pipeline, nil)
	vk.DestroyPipelineCache(d.device, d.pipelineCache, nil)
	vk.DestroyRenderPass(d.device, d.renderPass, nil)
	vk.DestroyPipelineLayout(d.device, d.pipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(d.device, d.descLayout, nil)

	for i := 0; i < demoTextureCount; i++ {
		vk.DestroyImageView(d.device, d.textures[i].view, nil)
		vk.DestroyImage(d.device, d.textures[i].image, nil)
		vk.FreeMemory(d.device, d.textures[i].mem, nil)
		vk.DestroySampler(d.device, d.textures[i].sampler, nil)
	}
	vk.DestroySwapchain(d.device, d.swapchain, nil)
	vk.DestroyImageView(d.device, d.depth.view, nil)
	vk.DestroyImage(d.device, d.depth.image, nil)
	vk.FreeMemory(d.device, d.depth.mem, nil)

	vk.DestroyBuffer(d.device, d.uniform.buf, nil)
	vk.FreeMemory(d.device, d.uniform.mem, nil)

	for i := 0; i < d.swapchainImageCount; i++ {
		vk.DestroyImageView(d.device, d.buffers[i].view, nil)
		vk.FreeCommandBuffers(d.device, d.cmdPool, 1, []vk.CommandBuffer{
			d.buffers[i].cmd,
		})
	}
	d.buffers = nil
	d.queueProps = nil

	vk.DestroyCommandPool(d.device, d.cmdPool, nil)
	vk.DestroyDevice(d.device, nil)

	if enableDebug {
		vk.DestroyDebugReportCallback(d.instance, d.dbgCallback, nil)
	}
	vk.DestroySurface(d.instance, d.surface, nil)
	vk.DestroyInstance(d.instance, nil)
}

func (d *Demo) resize() {
	if !d.prepared {
		return
	}

	// In order to properly resize the window, we must re-create the swapchain
	// AND redo the command buffers, etc.
	//
	// First, perform part of the Cleanup() function:
	d.prepared = false

	d.prepared = false
	for i := 0; i < d.swapchainImageCount; i++ {
		vk.DestroyFramebuffer(d.device, d.framebuffers[i], nil)
	}
	d.framebuffers = nil

	vk.DestroyDescriptorPool(d.device, d.descPool, nil)
	vk.DestroyPipeline(d.device, d.pipeline, nil)
	vk.DestroyPipelineCache(d.device, d.pipelineCache, nil)
	vk.DestroyRenderPass(d.device, d.renderPass, nil)
	vk.DestroyPipelineLayout(d.device, d.pipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(d.device, d.descLayout, nil)

	for i := 0; i < demoTextureCount; i++ {
		vk.DestroyImageView(d.device, d.textures[i].view, nil)
		vk.DestroyImage(d.device, d.textures[i].image, nil)
		vk.FreeMemory(d.device, d.textures[i].mem, nil)
		vk.DestroySampler(d.device, d.textures[i].sampler, nil)
	}
	vk.DestroyImageView(d.device, d.depth.view, nil)
	vk.DestroyImage(d.device, d.depth.image, nil)
	vk.FreeMemory(d.device, d.depth.mem, nil)

	vk.DestroyBuffer(d.device, d.uniform.buf, nil)
	vk.FreeMemory(d.device, d.uniform.mem, nil)

	for i := 0; i < d.swapchainImageCount; i++ {
		vk.DestroyImageView(d.device, d.buffers[i].view, nil)
		vk.FreeCommandBuffers(d.device, d.cmdPool, 1, []vk.CommandBuffer{
			d.buffers[i].cmd,
		})
	}
	d.buffers = nil

	// Second, re-perform the Prepare() function, which will re-create the
	// swapchain:
	d.Prepare(d.vsName, d.fsName, d.texName)
}

func (d *Demo) InitModel() {
	var eye = &linmath.Vec3{0, 3, 5}
	var origin = &linmath.Vec3{0, 0, 0}
	var up = &linmath.Vec3{0, 1, 0}

	d.spinAngle = 0.01
	d.spinIncrement = 0.01

	d.projectionMat = new(linmath.Mat4x4)
	d.viewMat = new(linmath.Mat4x4)
	d.modelMat = new(linmath.Mat4x4)

	fov := linmath.DegreesToRadians(45)
	d.projectionMat.Perspective(fov, 1, 0.1, 100)
	d.viewMat.LookAt(eye, origin, up)
	d.modelMat.Identity()
}

func (d *Demo) Step() {
	vk.DeviceWaitIdle(d.device)
	d.updateDataBuffer()
	d.draw()
	vk.DeviceWaitIdle(d.device)
}

func NewDemoForAndroid(appInfo vk.ApplicationInfo, window *android.NativeWindow) (d Demo) {
	existingExtensions := getInstanceExtensions()
	log.Println("[INFO] Instance extensions:", existingExtensions)

	instanceExtensions := []string{
		"VK_KHR_surface\x00",
		"VK_KHR_android_surface\x00",
	}
	if enableDebug {
		instanceExtensions = append(
			instanceExtensions, "VK_EXT_debug_report\x00")
	}

	existingLayers := getInstanceLayers()
	log.Println("[INFO] Instance layers:", existingLayers)

	// these layers must be included in APK,
	// see Android.mk and ValidationLayers.mk
	instanceLayers := []string{
		"VK_LAYER_GOOGLE_threading\x00",
		"VK_LAYER_LUNARG_parameter_validation\x00",
		"VK_LAYER_LUNARG_object_tracker\x00",
		"VK_LAYER_LUNARG_core_validation\x00",
		"VK_LAYER_LUNARG_api_dump\x00",
		"VK_LAYER_LUNARG_image\x00",
		"VK_LAYER_LUNARG_swapchain\x00",
		"VK_LAYER_GOOGLE_unique_objects\x00",
	}

	instanceInfo := vk.InstanceCreateInfo{
		SType:                   vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo:        &appInfo,
		EnabledExtensionCount:   uint32(len(instanceExtensions)),
		PpEnabledExtensionNames: instanceExtensions,
		EnabledLayerCount:       uint32(len(instanceLayers)),
		PpEnabledLayerNames:     instanceLayers,
	}
	err := vk.CreateInstance(&instanceInfo, nil, &d.instance)
	orPanic(err)

	surfaceCreateInfo := vk.AndroidSurfaceCreateInfo{
		SType:  vk.StructureTypeAndroidSurfaceCreateInfo,
		Window: (*vk.ANativeWindow)(window),
	}
	err = vk.CreateAndroidSurface(d.instance, &surfaceCreateInfo, nil, &d.surface)
	orPanic(err)

	gpuDevices := getPhysicalDevices(d.instance)
	d.gpu = gpuDevices[0] // choose the firts GPU available

	existingExtensions = getDeviceExtensions(d.gpu)
	log.Println("[INFO] Device extensions:", existingExtensions)

	existingLayers = getDeviceLayers(d.gpu)
	log.Println("[INFO] Device layers:", existingLayers)

	// these layers must be included in APK,
	// see Android.mk and ValidationLayers.mk
	deviceLayers := []string{
		"VK_LAYER_GOOGLE_threading\x00",
		"VK_LAYER_LUNARG_parameter_validation\x00",
		"VK_LAYER_LUNARG_object_tracker\x00",
		"VK_LAYER_LUNARG_core_validation\x00",
		"VK_LAYER_LUNARG_api_dump\x00",
		"VK_LAYER_LUNARG_image\x00",
		"VK_LAYER_LUNARG_swapchain\x00",
		"VK_LAYER_GOOGLE_unique_objects\x00",
	}

	deviceQueueInfos := []vk.DeviceQueueCreateInfo{{
		SType:            vk.StructureTypeDeviceQueueCreateInfo,
		QueueCount:       1,
		PQueuePriorities: []float32{1.0},
	}}
	deviceExtensions := []string{
		"VK_KHR_swapchain\x00",
	}
	deviceInfo := vk.DeviceCreateInfo{
		SType:                   vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount:    1,
		PQueueCreateInfos:       deviceQueueInfos,
		EnabledExtensionCount:   uint32(len(deviceExtensions)),
		PpEnabledExtensionNames: deviceExtensions,
		EnabledLayerCount:       uint32(len(deviceLayers)),
		PpEnabledLayerNames:     deviceLayers,
	}
	err = vk.CreateDevice(d.gpu, &deviceInfo, nil, &d.device)
	orPanic(err)

	if enableDebug {
		dbgCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
			PfnCallback: dbgCallbackFunc,
		}
		err = vk.CreateDebugReportCallback(d.instance, &dbgCreateInfo, nil, &d.dbgCallback)
		check(err, "vk.CreateDebugReportCallback failed with")
	}
	return d
}

func (d *Demo) prepareSurfaceCapabilities() {
	var caps vk.SurfaceCapabilities
	err := vk.GetPhysicalDeviceSurfaceCapabilities(d.gpu, d.surface, &caps)
	orPanic(err)

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(d.gpu, d.surface, &formatCount, nil)
	orPanicWith(formatCount > 0, "no surface formats available")
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(d.gpu, d.surface, &formatCount, formats)

	log.Println("[INFO] got", formatCount, "physical device surface formats")

	chosenFormat := -1
	for i := 0; i < int(formatCount); i++ {
		formats[i].Deref()
		if formats[i].Format == vk.FormatR8g8b8a8Unorm {
			chosenFormat = i
			break
		}
	}
	orPanicWith(chosenFormat != -1, "vk.FormatR8g8b8a8Unorm is not supported")

	caps.Deref()
	caps.CurrentExtent.Deref()
	d.format = formats[chosenFormat].Format
	d.colorSpace = formats[chosenFormat].ColorSpace
	d.width = caps.CurrentExtent.Width
	d.height = caps.CurrentExtent.Height

	for i := range formats {
		formats[i].Free()
	}
}

func getInstanceLayers() (layerNames []string) {
	var instanceLayerLen uint32
	err := vk.EnumerateInstanceLayerProperties(&instanceLayerLen, nil)
	orPanic(err)
	instanceLayers := make([]vk.LayerProperties, instanceLayerLen)
	err = vk.EnumerateInstanceLayerProperties(&instanceLayerLen, instanceLayers)
	orPanic(err)
	for _, layer := range instanceLayers {
		layer.Deref()
		layerNames = append(layerNames,
			vk.ToString(layer.LayerName[:]))
	}
	return layerNames
}

func getDeviceLayers(gpu vk.PhysicalDevice) (layerNames []string) {
	var deviceLayerLen uint32
	err := vk.EnumerateDeviceLayerProperties(gpu, &deviceLayerLen, nil)
	orPanic(err)
	deviceLayers := make([]vk.LayerProperties, deviceLayerLen)
	err = vk.EnumerateDeviceLayerProperties(gpu, &deviceLayerLen, deviceLayers)
	orPanic(err)
	for _, layer := range deviceLayers {
		layer.Deref()
		layerNames = append(layerNames,
			vk.ToString(layer.LayerName[:]))
	}
	return layerNames
}

func getInstanceExtensions() (extNames []string) {
	var instanceExtLen uint32
	err := vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, nil)
	orPanic(err)
	instanceExt := make([]vk.ExtensionProperties, instanceExtLen)
	err = vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, instanceExt)
	orPanic(err)
	for _, ext := range instanceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
}

func getDeviceExtensions(gpu vk.PhysicalDevice) (extNames []string) {
	var deviceExtLen uint32
	err := vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, nil)
	orPanic(err)
	deviceExt := make([]vk.ExtensionProperties, deviceExtLen)
	err = vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, deviceExt)
	orPanic(err)
	for _, ext := range deviceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
}

func getPhysicalDevices(instance vk.Instance) []vk.PhysicalDevice {
	var gpuCount uint32
	err := vk.EnumeratePhysicalDevices(instance, &gpuCount, nil)
	orPanic(err)
	orPanicWith(gpuCount != 0, "getPhysicalDevice: no GPUs found on the system")

	gpuList := make([]vk.PhysicalDevice, gpuCount)
	err = vk.EnumeratePhysicalDevices(instance, &gpuCount, gpuList)
	orPanic(err)
	return gpuList
}
