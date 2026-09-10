// Windows 屏幕捕获：GDI BitBlt 抓取整个虚拟桌面，绘制光标位置标记，缩放后输出 JPEG。
package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procGetCursorInfo      = user32.NewProc("GetCursorInfo")
	procSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatBmp    = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
	srccopy           = 0x00CC0020
	dibRGBColors      = 0
	cursorShowing     = 1
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32 // 负值 = 自上而下的行序
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [2]uint32
}

type point struct{ X, Y int32 }

type cursorInfo struct {
	CBSize uint32
	Flags  uint32
	Cursor syscall.Handle
	Pos    point
}

func init() {
	// 使用真实像素，避免 DPI 虚拟化导致截屏模糊、坐标偏移
	procSetProcessDPIAware.Call()
}

// virtualScreenRect 返回虚拟桌面（所有显示器）的范围。
func virtualScreenRect() (vx, vy, vw, vh int) {
	ret := func(idx int) int {
		r, _, _ := procGetSystemMetrics.Call(uintptr(idx))
		return int(int32(r))
	}
	vx, vy = ret(smXVirtualScreen), ret(smYVirtualScreen)
	vw, vh = ret(smCXVirtualScreen), ret(smCYVirtualScreen)
	if vw <= 0 || vh <= 0 {
		vx, vy = 0, 0
		vw, vh = ret(0), ret(1) // 主屏
	}
	return
}

var (
	captureMu  sync.Mutex
	captureBuf []byte
)

// CaptureJPEG 截取当前屏幕（含光标标记），按需缩放到 targetW 宽度，返回 JPEG 数据。
// 返回值中的 vw/vh 是未缩放的虚拟桌面尺寸，控制端用它做坐标换算。
func CaptureJPEG(targetW, quality int) ([]byte, int, int, error) {
	captureMu.Lock()
	defer captureMu.Unlock()

	vx, vy, vw, vh := virtualScreenRect()
	if vw <= 0 || vh <= 0 {
		return nil, 0, 0, errors.New("无法获取屏幕尺寸")
	}

	need := vw * vh * 4
	if len(captureBuf) < need {
		captureBuf = make([]byte, need)
	}
	buf := captureBuf[:need]

	if err := gdiCapture(buf, vx, vy, vw, vh); err != nil {
		return nil, 0, 0, err
	}

	img := &image.RGBA{Pix: buf, Stride: vw * 4, Rect: image.Rect(0, 0, vw, vh)}
	drawCursorDot(img, vx, vy)

	if targetW > 0 && targetW < vw {
		img = scaleNearest(img, targetW)
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), vw, vh, nil
}

func gdiCapture(buf []byte, vx, vy, vw, vh int) error {
	hScreen, _, _ := procGetDC.Call(0)
	if hScreen == 0 {
		return errors.New("GetDC 失败")
	}
	defer procReleaseDC.Call(0, hScreen)

	hMem, _, _ := procCreateCompatibleDC.Call(hScreen)
	if hMem == 0 {
		return errors.New("CreateCompatibleDC 失败")
	}
	defer procDeleteDC.Call(hMem)

	hBmp, _, _ := procCreateCompatBmp.Call(hScreen, uintptr(vw), uintptr(vh))
	if hBmp == 0 {
		return errors.New("CreateCompatibleBitmap 失败")
	}
	defer procDeleteObject.Call(hBmp)

	oldObj, _, _ := procSelectObject.Call(hMem, hBmp)
	defer procSelectObject.Call(hMem, oldObj)

	ok, _, callErr := procBitBlt.Call(hMem,
		0, 0, uintptr(vw), uintptr(vh),
		hScreen, uintptr(vx), uintptr(vy), srccopy)
	if ok == 0 {
		return errors.New("BitBlt 失败: " + callErr.Error())
	}

	// GetDIBits 要求位图未选入 DC
	procSelectObject.Call(hMem, oldObj)

	bmi := bitmapInfo{}
	bmi.Header.Size = uint32(unsafe.Sizeof(bmi.Header))
	bmi.Header.Width = int32(vw)
	bmi.Header.Height = int32(-vh)
	bmi.Header.Planes = 1
	bmi.Header.BitCount = 32
	bmi.Header.Compression = 0 // BI_RGB

	ok, _, callErr = procGetDIBits.Call(hScreen, hBmp, 0, uintptr(vh),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bmi)), dibRGBColors)
	if ok == 0 {
		return errors.New("GetDIBits 失败: " + callErr.Error())
	}

	// BGRA -> RGBA，并补上不透明 alpha
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+2] = buf[i+2], buf[i]
		buf[i+3] = 255
	}
	return nil
}

// drawCursorDot 在图像上画出系统光标当前位置（红色圆点），让控制端看得到真实指针。
func drawCursorDot(img *image.RGBA, vx, vy int) {
	var ci cursorInfo
	ci.CBSize = uint32(unsafe.Sizeof(ci))
	r, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci)))
	if r == 0 || ci.Flags != cursorShowing {
		return
	}
	cx := int(ci.Pos.X) - vx
	cy := int(ci.Pos.Y) - vy
	if cx < 0 || cy < 0 || cx >= img.Rect.Dx() || cy >= img.Rect.Dy() {
		return
	}
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			d2 := dx*dx + dy*dy
			if d2 > 64 {
				continue
			}
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			off := img.PixOffset(x, y)
			if d2 <= 25 { // 内圈红
				img.Pix[off], img.Pix[off+1], img.Pix[off+2] = 230, 60, 60
			} else { // 外圈白，保证任意背景下可见
				img.Pix[off], img.Pix[off+1], img.Pix[off+2] = 255, 255, 255
			}
			img.Pix[off+3] = 255
		}
	}
}

func scaleNearest(src *image.RGBA, targetW int) *image.RGBA {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	targetH := sh * targetW / sw
	if targetH < 1 {
		targetH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		sy := y * sh / targetH
		for x := 0; x < targetW; x++ {
			sx := x * sw / targetW
			sOff := src.PixOffset(sx, sy)
			dOff := dst.PixOffset(x, y)
			copy(dst.Pix[dOff:dOff+4], src.Pix[sOff:sOff+4])
		}
	}
	return dst
}
