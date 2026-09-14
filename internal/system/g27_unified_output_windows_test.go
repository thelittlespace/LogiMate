//go:build windows

package system

import (
	"syscall"
	"testing"
)

func TestOpenNativeHIDTransportBorrowsUnifiedG27Session(t *testing.T) {
	path := `\\?\HID#VID_046D&PID_C29B#7&BUILD010&0&0000#{00001124-0000-1000-8000-00805F9B34FB}`
	fake := syscall.Handle(0x1234)

	g27Shared.Lock()
	oldPath, oldHandle, oldGeneration := g27Shared.path, g27Shared.handle, g27Shared.generation
	oldNextOpen, oldFailures := g27Shared.nextOpen, g27Shared.failures
	g27Shared.path, g27Shared.handle, g27Shared.generation = path, fake, 77
	g27Shared.nextOpen = oldNextOpen
	g27Shared.failures = 0
	g27Shared.Unlock()
	defer func() {
		g27Shared.Lock()
		g27Shared.path, g27Shared.handle, g27Shared.generation = oldPath, oldHandle, oldGeneration
		g27Shared.nextOpen, g27Shared.failures = oldNextOpen, oldFailures
		g27Shared.Unlock()
	}()

	tr, err := openNativeHIDTransport(path)
	if err != nil {
		t.Fatalf("open unified transport: %v", err)
	}
	if !tr.borrowedG27 {
		t.Fatal("expected borrowed G27 session")
	}
	if got := tr.Backend(); got != "g27-shared-overlapped" {
		t.Fatalf("backend=%q", got)
	}
	if got := tr.ShareMode(); got != "shared-g27-session" {
		t.Fatalf("share mode=%q", got)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("close wrapper: %v", err)
	}

	g27Shared.RLock()
	stillOpen := g27Shared.handle == fake && g27Shared.generation == 77 && g27Shared.path == path
	g27Shared.RUnlock()
	if !stillOpen {
		t.Fatal("closing borrowed output wrapper must not close the input-owned G27 session")
	}
}
