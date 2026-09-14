//go:build windows

package app

import (
	"encoding/binary"
	"math"
	"sync"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/brand"
)

var (
	brandIconMu sync.Mutex
	brandIcons  = map[int32]HICON{}
)

// brandIcon loads the nearest image from the embedded multi-resolution ICO.
// The icon is created once per requested size and reused for the process lifetime.
func brandIcon(size int32) HICON {
	if size <= 0 {
		size = 32
	}
	brandIconMu.Lock()
	defer brandIconMu.Unlock()
	if h := brandIcons[size]; h != 0 {
		return h
	}
	data := brand.IconICO
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestOff, bestLen, bestDelta := 0, 0, math.MaxInt
	for i := 0; i < count; i++ {
		off := 6 + i*16
		if off+16 > len(data) {
			break
		}
		w := int(data[off])
		h := int(data[off+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		n := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
		imgOff := int(binary.LittleEndian.Uint32(data[off+12 : off+16]))
		if n <= 0 || imgOff < 0 || imgOff+n > len(data) {
			continue
		}
		d := absInt(w-int(size)) + absInt(h-int(size))
		if d < bestDelta {
			bestOff, bestLen, bestDelta = imgOff, n, d
		}
	}
	if bestLen == 0 {
		return 0
	}
	r, _, _ := pCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&data[bestOff])), uintptr(bestLen), 1, 0x00030000,
		uintptr(size), uintptr(size), LR_DEFAULTCOLOR,
	)
	h := HICON(r)
	if h != 0 {
		brandIcons[size] = h
	}
	return h
}

func drawBrandIcon(hdc uintptr, r RECT) {
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w <= 0 || h <= 0 {
		return
	}
	size := w
	if h < size {
		size = h
	}
	icon := brandIcon(size)
	if icon == 0 {
		return
	}
	x := r.Left + (w-size)/2
	y := r.Top + (h-size)/2
	pDrawIconEx.Call(hdc, uintptr(x), uintptr(y), uintptr(icon), uintptr(size), uintptr(size), 0, 0, DI_NORMAL)
}

func releaseBrandIcons() {
	brandIconMu.Lock()
	defer brandIconMu.Unlock()
	for _, h := range brandIcons {
		if h != 0 {
			pDestroyIcon.Call(uintptr(h))
		}
	}
	brandIcons = map[int32]HICON{}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
