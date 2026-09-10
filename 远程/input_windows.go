// Windows 鼠标/键盘模拟（SendInput）与剪贴板写入，全部走原生 API。
package main

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	procSendInput     = user32.NewProc("SendInput")
	procVkKeyScanW    = user32.NewProc("VkKeyScanW")
	procMapVirtualKey = user32.NewProc("MapVirtualKeyW")

	procOpenClipboard   = user32.NewProc("OpenClipboard")
	procCloseClipboard  = user32.NewProc("CloseClipboard")
	procEmptyClipboard  = user32.NewProc("EmptyClipboard")
	procSetClipboard    = user32.NewProc("SetClipboardData")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGlobalAlloc     = kernel32.NewProc("GlobalAlloc")
	procGlobalLock      = kernel32.NewProc("GlobalLock")
	procGlobalUnlock    = kernel32.NewProc("GlobalUnlock")
	procGlobalFree      = kernel32.NewProc("GlobalFree")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseEventfMove       = 0x0001
	mouseEventfLeftDown   = 0x0002
	mouseEventfLeftUp     = 0x0004
	mouseEventfRightDown  = 0x0008
	mouseEventfRightUp    = 0x0010
	mouseEventfMiddleDown = 0x0020
	mouseEventfMiddleUp   = 0x0040
	mouseEventfWheel      = 0x0800
	mouseEventfHWheel     = 0x1000
	mouseEventfVirtualDesk = 0x4000
	mouseEventfAbsolute   = 0x8000

	keyEventfKeyup   = 0x0002
	keyEventfUnicode = 0x0004

	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// 与 C 端 INPUT 结构在 amd64 上的内存布局一致（8 字节头 + 32 字节联合体，共 40 字节）
type mouseInputData struct {
	Dx, Dy       int32
	MouseData    uint32
	DwFlags      uint32
	Time         uint32
	ExtraInfo    uintptr
}

type keybdInputData struct {
	WVk, WScan  uint16
	DwFlags     uint32
	Time        uint32
	ExtraInfo   uintptr
}

type winInput struct {
	Type uint32
	_    uint32
	Mi   mouseInputData // 联合体中最大的成员；键盘事件通过指针转型写入
}

var vkNames = map[string]uint16{
	"enter": 0x0D, "return": 0x0D, "tab": 0x09, "esc": 0x1B, "escape": 0x1B,
	"space": 0x20, "backspace": 0x08, "delete": 0x2E, "del": 0x2E,
	"insert": 0x2D, "ins": 0x2D, "home": 0x24, "end": 0x23,
	"pageup": 0x21, "pagedown": 0x22, "prior": 0x21, "next": 0x22,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"win": 0x5B, "lwin": 0x5B, "rwin": 0x5C, "capslock": 0x14,
	"printscreen": 0x2C, "pause": 0x13, "contextmenu": 0x5D,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
}

var vkMods = map[string]uint16{
	"ctrl": 0x11, "control": 0x11, "alt": 0x12, "shift": 0x10, "win": 0x5B, "meta": 0x5B,
}

// MouseMove 把光标移动到虚拟屏幕坐标 (x, y)。
func MouseMove(x, y int) error {
	vx, vy, vw, vh := virtualScreenRect()
	if vw <= 1 || vh <= 1 {
		return errors.New("屏幕尺寸无效")
	}
	if x < vx {
		x = vx
	}
	if y < vy {
		y = vy
	}
	if x > vx+vw-1 {
		x = vx + vw - 1
	}
	if y > vy+vh-1 {
		y = vy + vh - 1
	}
	in := winInput{Type: inputMouse}
	in.Mi.Dx = int32((x - vx) * 65535 / (vw - 1))
	in.Mi.Dy = int32((y - vy) * 65535 / (vh - 1))
	in.Mi.DwFlags = mouseEventfMove | mouseEventfAbsolute | mouseEventfVirtualDesk
	return sendInputs([]winInput{in})
}

// MouseButton 按下或释放鼠标按键，button 为 left/right/middle。
func MouseButton(button string, down bool) error {
	var downFlag, upFlag uint32
	switch button {
	case "left":
		downFlag, upFlag = mouseEventfLeftDown, mouseEventfLeftUp
	case "right":
		downFlag, upFlag = mouseEventfRightDown, mouseEventfRightUp
	case "middle":
		downFlag, upFlag = mouseEventfMiddleDown, mouseEventfMiddleUp
	default:
		return fmt.Errorf("未知鼠标按键: %s", button)
	}
	in := winInput{Type: inputMouse}
	if down {
		in.Mi.DwFlags = downFlag
	} else {
		in.Mi.DwFlags = upFlag
	}
	return sendInputs([]winInput{in})
}

// MouseWheel 滚动滚轮，amount 为 Windows 滚轮增量（120 = 一格），正数向上/向左。
func MouseWheel(amount int, horiz bool) error {
	in := winInput{Type: inputMouse}
	if horiz {
		in.Mi.DwFlags = mouseEventfHWheel
	} else {
		in.Mi.DwFlags = mouseEventfWheel
	}
	in.Mi.MouseData = uint32(int32(amount))
	return sendInputs([]winInput{in})
}

// SendKeyCombo 按下并释放一个按键（可带修饰键），key 为键名或单个字符。
func SendKeyCombo(key string, mods []string) error {
	vk, extraMods, err := resolveVK(strings.ToLower(key))
	if err != nil {
		return err
	}
	var modVKs []uint16
	seen := map[uint16]bool{}
	for _, m := range append(mods, extraMods...) {
		v, ok := vkMods[strings.ToLower(strings.TrimSpace(m))]
		if !ok {
			return fmt.Errorf("未知修饰键: %s", m)
		}
		if !seen[v] {
			seen[v] = true
			modVKs = append(modVKs, v)
		}
	}

	var inputs []winInput
	for _, v := range modVKs {
		inputs = append(inputs, keyInput(v, 0))
	}
	scanRet, _, _ := procMapVirtualKey.Call(uintptr(vk), 0)
	scan := uint16(scanRet)
	inputs = append(inputs, keyInput(vk, scan), keyInputUp(vk, scan))
	for i := len(modVKs) - 1; i >= 0; i-- {
		inputs = append(inputs, keyInputUp(modVKs[i], 0))
	}
	return sendInputs(inputs)
}

// SendText 以 Unicode 方式输入文本（支持中文），\n 发送回车键。
func SendText(text string) error {
	if text == "" {
		return nil
	}
	var inputs []winInput
	const chunk = 200 // 每次 SendInput 的事件数上限（分块发送）
	for _, u := range utf16.Encode([]rune(text)) {
		switch {
		case u == '\n':
			inputs = append(inputs, keyInput(0x0D, 0), keyInputUp(0x0D, 0))
		case u == '\t':
			inputs = append(inputs, keyInput(0x09, 0), keyInputUp(0x09, 0))
		case u == '\r':
			continue
		default:
			inputs = append(inputs,
				keyInputUnicode(u, keyEventfUnicode),
				keyInputUnicode(u, keyEventfUnicode|keyEventfKeyup))
		}
		if len(inputs) >= chunk {
			if err := sendInputs(inputs); err != nil {
				return err
			}
			inputs = nil
		}
	}
	if len(inputs) > 0 {
		return sendInputs(inputs)
	}
	return nil
}

// PasteText 写入剪贴板后发送 Ctrl+V，比逐字符输入快得多，适合长文本。
func PasteText(text string) error {
	if text == "" {
		return nil
	}
	if err := setClipboardText(text); err != nil {
		return err
	}
	return SendKeyCombo("v", []string{"ctrl"})
}

func setClipboardText(text string) error {
	u16 := utf16.Encode([]rune(text))
	size := (len(u16) + 1) * 2

	h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return errors.New("分配剪贴板内存失败")
	}
	pRet, _, _ := procGlobalLock.Call(h)
	if pRet == 0 {
		procGlobalFree.Call(h)
		return errors.New("锁定剪贴板内存失败")
	}
	// GlobalLock 返回的是线性地址，还原成指针后写入
	p := *(*unsafe.Pointer)(unsafe.Pointer(&pRet))
	dst := unsafe.Slice((*byte)(p), size)
	for i, u := range u16 {
		dst[i*2] = byte(u)
		dst[i*2+1] = byte(u >> 8)
	}
	dst[len(u16)*2] = 0
	dst[len(u16)*2+1] = 0
	procGlobalUnlock.Call(h)

	for i := 0; i < 10; i++ {
		if opened, _, _ := procOpenClipboard.Call(0); opened != 0 {
			procEmptyClipboard.Call()
			set, _, _ := procSetClipboard.Call(cfUnicodeText, h)
			procCloseClipboard.Call()
			if set == 0 {
				procGlobalFree.Call(h)
				return errors.New("写入剪贴板失败")
			}
			return nil // 成功后内存归系统所有，不能 GlobalFree
		}
		time.Sleep(30 * time.Millisecond)
	}
	procGlobalFree.Call(h)
	return errors.New("剪贴板被其他程序占用")
}

func resolveVK(key string) (uint16, []string, error) {
	if key == "" {
		return 0, nil, errors.New("按键为空")
	}
	if vk, ok := vkNames[key]; ok {
		return vk, nil, nil
	}
	if len(key) == 1 {
		return vkFromChar(key)
	}
	// 兼容浏览器传来的 KeyC / Digit1 形式
	if len(key) == 4 && strings.HasPrefix(key, "key") {
		if vk, _, err := vkFromChar(strings.ToLower(key[3:])); err == nil {
			return vk, nil, nil
		}
	}
	if len(key) == 6 && strings.HasPrefix(key, "digit") {
		if vk, _, err := vkFromChar(key[5:]); err == nil {
			return vk, nil, nil
		}
	}
	return 0, nil, fmt.Errorf("无法识别按键: %s", key)
}

func vkFromChar(ch string) (uint16, []string, error) {
	r := []rune(ch)
	if len(r) != 1 {
		return 0, nil, fmt.Errorf("无法识别按键: %s", ch)
	}
	if r[0] >= 'a' && r[0] <= 'z' {
		return uint16(r[0] - 'a' + 0x41), nil, nil // VK_A..VK_Z
	}
	if r[0] >= '0' && r[0] <= '9' {
		return uint16(r[0] - '0' + 0x30), nil, nil // VK_0..VK_9
	}
	ret, _, _ := procVkKeyScanW.Call(uintptr(r[0]))
	sc := int16(uint16(ret & 0xFFFF))
	if sc == -1 {
		return 0, nil, fmt.Errorf("无法识别按键: %s", ch)
	}
	vk := uint16(sc & 0xFF)
	var mods []string
	shift := (sc >> 8) & 1
	ctrl := (sc >> 9) & 1
	alt := (sc >> 10) & 1
	if shift != 0 {
		mods = append(mods, "shift")
	}
	if ctrl != 0 {
		mods = append(mods, "ctrl")
	}
	if alt != 0 {
		mods = append(mods, "alt")
	}
	return vk, mods, nil
}

func keyInput(vk, scan uint16) winInput {
	in := winInput{Type: inputKeyboard}
	k := (*keybdInputData)(unsafe.Pointer(&in.Mi))
	k.WVk, k.WScan = vk, scan
	return in
}

func keyInputUp(vk, scan uint16) winInput {
	in := keyInput(vk, scan)
	(*keybdInputData)(unsafe.Pointer(&in.Mi)).DwFlags = keyEventfKeyup
	return in
}

func keyInputUnicode(unit uint16, flags uint32) winInput {
	in := winInput{Type: inputKeyboard}
	k := (*keybdInputData)(unsafe.Pointer(&in.Mi))
	k.WScan, k.DwFlags = unit, flags
	return in
}

func sendInputs(inputs []winInput) error {
	if len(inputs) == 0 {
		return nil
	}
	ret, _, callErr := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(winInput{}))
	if ret == 0 {
		return fmt.Errorf("SendInput 失败: %v", callErr)
	}
	return nil
}
