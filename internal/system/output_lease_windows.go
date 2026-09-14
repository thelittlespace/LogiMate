//go:build windows

package system

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32Lease             = syscall.NewLazyDLL("kernel32.dll")
	pCreateMutexWLease        = kernel32Lease.NewProc("CreateMutexW")
	pWaitForSingleObjectLease = kernel32Lease.NewProc("WaitForSingleObject")
	pReleaseMutexLease        = kernel32Lease.NewProc("ReleaseMutex")
)

const (
	waitObject0   = 0x00000000
	waitAbandoned = 0x00000080
	waitTimeout   = 0x00000102
)

type nativeOutputLeaseToken struct {
	Generation uint64
	WheelID    string
	SessionID  string
	Model      string
	Path       string
	Purpose    string
	Motor      bool
	DataDir    string
	AcquiredAt time.Time
	mutex      syscall.Handle
}

type NativeOutputLeaseStatus struct {
	Active     bool
	Generation uint64
	WheelID    string
	SessionID  string
	Model      string
	Purpose    string
	Motor      bool
	DataDir    string
	AcquiredAt time.Time
}

var nativeOutputLease = struct {
	sync.Mutex
	generation uint64
	token      *nativeOutputLeaseToken
}{}

func acquireNamedMutex(name string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	h, _, callErr := pCreateMutexWLease.Call(0, 0, uintptr(unsafe.Pointer(p)))
	if h == 0 {
		return syscall.InvalidHandle, fmt.Errorf("CreateMutexW(%s): %w", name, callErr)
	}
	handle := syscall.Handle(h)
	w, _, waitErr := pWaitForSingleObjectLease.Call(h, 0)
	switch w {
	case waitObject0, waitAbandoned:
		return handle, nil
	case waitTimeout:
		_ = syscall.CloseHandle(handle)
		return syscall.InvalidHandle, errors.New("eine andere LogiMate-Instanz besitzt bereits den nativen Hardware-Output")
	default:
		_ = syscall.CloseHandle(handle)
		return syscall.InvalidHandle, fmt.Errorf("WaitForSingleObject(%s) fehlgeschlagen: %w", name, waitErr)
	}
}

func releaseNamedMutex(h syscall.Handle) {
	if h == 0 || h == syscall.InvalidHandle {
		return
	}
	_, _, _ = pReleaseMutexLease.Call(uintptr(h))
	_ = syscall.CloseHandle(h)
}

// AcquireApplicationInstanceLock prevents two full LogiMate GUI processes from
// racing state/migration/output ownership. The updater helper runs before this
// lock is acquired and therefore remains able to replace a stopped instance.
func AcquireApplicationInstanceLock() (func(), error) {
	h, err := acquireNamedMutex(`Global\LogiMate.App.Singleton.v1`)
	if err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { releaseNamedMutex(h) }) }, nil
}

func acquireNativeOutputLease(s State, purpose string, motor bool) (nativeOutputLeaseToken, error) {
	w, path, err := validateNativeOutputTarget(s)
	if err != nil {
		return nativeOutputLeaseToken{}, err
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = "native-output"
	}

	nativeOutputLease.Lock()
	if nativeOutputLease.token != nil {
		t := nativeOutputLease.token
		nativeOutputLease.Unlock()
		return nativeOutputLeaseToken{}, fmt.Errorf("nativer Output ist bereits belegt: %s (%s)", t.Purpose, t.WheelID)
	}
	h, err := acquireNamedMutex(`Global\LogiMate.NativeOutput.v1`)
	if err != nil {
		nativeOutputLease.Unlock()
		return nativeOutputLeaseToken{}, err
	}
	nativeOutputLease.generation++
	t := nativeOutputLeaseToken{
		Generation: nativeOutputLease.generation,
		WheelID:    w.ID,
		SessionID:  w.SessionID,
		Model:      w.Model,
		Path:       path,
		Purpose:    purpose,
		Motor:      motor,
		DataDir:    s.DataDir,
		AcquiredAt: time.Now(),
		mutex:      h,
	}
	nativeOutputLease.token = &t
	nativeOutputLease.Unlock()
	updateNativeWheelOutput(t, "lease-acquired", "", true)
	return t, nil
}

func releaseNativeOutputLease(t nativeOutputLeaseToken) bool {
	nativeOutputLease.Lock()
	cur := nativeOutputLease.token
	if cur == nil || cur.Generation != t.Generation {
		nativeOutputLease.Unlock()
		return false
	}
	nativeOutputLease.token = nil
	nativeOutputLease.Unlock()
	releaseNamedMutex(cur.mutex)
	updateNativeWheelOutput(*cur, "lease-released", "", false)
	return true
}

func currentNativeOutputLease() (nativeOutputLeaseToken, bool) {
	nativeOutputLease.Lock()
	defer nativeOutputLease.Unlock()
	if nativeOutputLease.token == nil {
		return nativeOutputLeaseToken{}, false
	}
	return *nativeOutputLease.token, true
}

func NativeOutputLeaseSnapshot() NativeOutputLeaseStatus {
	t, ok := currentNativeOutputLease()
	if !ok {
		return NativeOutputLeaseStatus{}
	}
	return NativeOutputLeaseStatus{Active: true, Generation: t.Generation, WheelID: t.WheelID, SessionID: t.SessionID, Model: t.Model, Purpose: t.Purpose, Motor: t.Motor, DataDir: t.DataDir, AcquiredAt: t.AcquiredAt}
}
