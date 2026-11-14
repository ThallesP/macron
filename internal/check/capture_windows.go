//go:build windows

package check

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetWindowTextW     = user32.NewProc("GetWindowTextW")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procSetForeground      = user32.NewProc("SetForegroundWindow")
	procSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	procSetCursorPos       = user32.NewProc("SetCursorPos")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBM = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

type rect struct{ Left, Top, Right, Bottom int32 }
type point struct{ X, Y int32 }

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func init() {
	procSetProcessDPIAware.Call()
}

var (
	enumerated   []windowsWindow
	enumCallback = syscall.NewCallback(func(handle uintptr, _ uintptr) uintptr {
		if visible, _, _ := procIsWindowVisible.Call(handle); visible == 0 {
			return 1
		}
		buffer := make([]uint16, 512)
		procGetWindowTextW.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		enumerated = append(enumerated, windowsWindow{Handle: handle, Title: syscall.UTF16ToString(buffer)})
		return 1
	})
)

func parkCursor() error {
	if ok, _, err := procSetCursorPos.Call(0, 0); ok == 0 {
		return fmt.Errorf("SetCursorPos: %w", err)
	}
	return nil
}

func listWindowsWindows() ([]windowsWindow, error) {
	enumerated = nil
	procEnumWindows.Call(enumCallback, 0)
	return enumerated, nil
}

func captureClientArea(handle uintptr) (image.Image, error) {
	procSetForeground.Call(handle)
	var client rect
	if ok, _, err := procGetClientRect.Call(handle, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return nil, fmt.Errorf("GetClientRect: %w", err)
	}
	origin := point{}
	procClientToScreen.Call(handle, uintptr(unsafe.Pointer(&origin)))
	width, height := client.Right-client.Left, client.Bottom-client.Top
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("the window's client area is %dx%d", width, height)
	}

	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)
	memory, _, _ := procCreateCompatibleDC.Call(screen)
	defer procDeleteDC.Call(memory)
	bitmap, _, _ := procCreateCompatibleBM.Call(screen, uintptr(width), uintptr(height))
	defer procDeleteObject.Call(bitmap)
	procSelectObject.Call(memory, bitmap)
	const srccopy = 0x00CC0020
	if ok, _, err := procBitBlt.Call(memory, 0, 0, uintptr(width), uintptr(height), screen, uintptr(origin.X), uintptr(origin.Y), srccopy); ok == 0 {
		return nil, fmt.Errorf("BitBlt: %w", err)
	}

	header := bitmapInfoHeader{Width: width, Height: -height, Planes: 1, BitCount: 32}
	header.Size = uint32(unsafe.Sizeof(header))
	pixels := make([]byte, int(width)*int(height)*4)
	if lines, _, err := procGetDIBits.Call(memory, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0); lines == 0 {
		return nil, fmt.Errorf("GetDIBits: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for i := 0; i < len(pixels); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], 255
	}
	return img, nil
}
