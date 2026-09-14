//go:build windows

package system

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	kernel32Native = syscall.NewLazyDLL("kernel32.dll")
	setupapi       = syscall.NewLazyDLL("setupapi.dll")
	user32Native   = syscall.NewLazyDLL("user32.dll")
	hidNative      = syscall.NewLazyDLL("hid.dll")

	pRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	pRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	pRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	pRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	pRegDeleteTreeW   = advapi32.NewProc("RegDeleteTreeW")
	pRegCloseKey      = advapi32.NewProc("RegCloseKey")

	pCreateToolhelp32Snapshot       = kernel32Native.NewProc("CreateToolhelp32Snapshot")
	pProcess32FirstW                = kernel32Native.NewProc("Process32FirstW")
	pProcess32NextW                 = kernel32Native.NewProc("Process32NextW")
	pCloseHandle                    = kernel32Native.NewProc("CloseHandle")
	pOpenProcessNative              = kernel32Native.NewProc("OpenProcess")
	pQueryFullProcessImageNameW     = kernel32Native.NewProc("QueryFullProcessImageNameW")
	pMoveFileExW                    = kernel32Native.NewProc("MoveFileExW")
	pGetForegroundWindowNative      = user32Native.NewProc("GetForegroundWindow")
	pGetWindowThreadProcessIdNative = user32Native.NewProc("GetWindowThreadProcessId")

	pSetupDiGetClassDevsW              = setupapi.NewProc("SetupDiGetClassDevsW")
	pSetupDiEnumDeviceInfo             = setupapi.NewProc("SetupDiEnumDeviceInfo")
	pSetupDiGetDeviceInstanceIdW       = setupapi.NewProc("SetupDiGetDeviceInstanceIdW")
	pSetupDiGetDeviceRegistryPropertyW = setupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
	pSetupDiGetDevicePropertyW         = setupapi.NewProc("SetupDiGetDevicePropertyW")
	pSetupDiDestroyDeviceInfoList      = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	pSetupDiEnumDeviceInterfaces       = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	pSetupDiGetDeviceInterfaceDetailW  = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")

	pHidDGetAttributes     = hidNative.NewProc("HidD_GetAttributes")
	pHidDGetPreparsedData  = hidNative.NewProc("HidD_GetPreparsedData")
	pHidDFreePreparsedData = hidNative.NewProc("HidD_FreePreparsedData")
	pHidDGetProductString  = hidNative.NewProc("HidD_GetProductString")
	pHidDGetSerialNumber   = hidNative.NewProc("HidD_GetSerialNumberString")
	pHidPGetCaps           = hidNative.NewProc("HidP_GetCaps")

	pGetRawInputDeviceList  = user32Native.NewProc("GetRawInputDeviceList")
	pGetRawInputDeviceInfoW = user32Native.NewProc("GetRawInputDeviceInfoW")
)

const (
	hkeyCurrentUser  = uintptr(0x80000001)
	hkeyLocalMachine = uintptr(0x80000002)
	keyQueryValue    = 0x0001
	keySetValue      = 0x0002
	keyCreateSubKey  = 0x0004
	regSZ            = 1
	regDWORD         = 4

	th32csSnapProcess              = 0x00000002
	processQueryLimitedInformation = 0x1000
	invalidHandle                  = ^uintptr(0)

	digcfPresent         = 0x00000002
	digcfAllClasses      = 0x00000004
	digcfDeviceInterface = 0x00000010
	spdrpDeviceDesc      = 0x00000000
	spdrpService         = 0x00000004
	spdrpDriver          = 0x00000009
	spdrpFriendlyName    = 0x0000000C

	rimTypeHID     = 2
	ridiDeviceName = 0x20000007
)

type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type devPropKey struct {
	FmtID winGUID
	PID   uint32
}

var devpkeyDeviceDriverInfPath = devPropKey{
	FmtID: winGUID{Data1: 0xa8b865dd, Data2: 0x2e3d, Data3: 0x4094, Data4: [8]byte{0xad, 0x97, 0xe5, 0x93, 0xa7, 0x0c, 0x75, 0xd6}},
	PID:   5,
}

var devpkeyDeviceParent = devPropKey{
	// DEVPKEY_Device_Parent = {4340a6c5-93fa-4706-972c-7b648008a5a7}, PID 8.
	// 0.1.1 accidentally used the fmtid for a different property family here,
	// which prevented HID child nodes from resolving to their USB wheel parent.
	FmtID: winGUID{Data1: 0x4340a6c5, Data2: 0x93fa, Data3: 0x4706, Data4: [8]byte{0x97, 0x2c, 0x7b, 0x64, 0x80, 0x08, 0xa5, 0xa7}},
	PID:   8,
}

var devpkeyDeviceLocationPaths = devPropKey{
	// DEVPKEY_Device_LocationPaths = {a45c254e-df1c-4efd-8020-67d146a850e0}, PID 37.
	FmtID: winGUID{Data1: 0xa45c254e, Data2: 0xdf1c, Data3: 0x4efd, Data4: [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}},
	PID:   37,
}

var devpkeyDeviceContainerID = devPropKey{
	// DEVPKEY_Device_ContainerId = {8c7ed206-3f8a-4827-b3ab-ae9e1faefc6c}, PID 2.
	// Windows uses this GUID to group multiple devnodes that belong to one
	// physical device. It is therefore LogiMate's primary wheel identity.
	FmtID: winGUID{Data1: 0x8c7ed206, Data2: 0x3f8a, Data3: 0x4827, Data4: [8]byte{0xb3, 0xab, 0xae, 0x9e, 0x1f, 0xae, 0xfc, 0x6c}},
	PID:   2,
}

const (
	devPropTypeGUID       = 0x0000000D
	devPropTypeString     = 0x00000012
	devPropTypeStringList = 0x00002012
)

type spDevInfoData struct {
	CbSize    uint32
	ClassGuid [16]byte
	DevInst   uint32
	Reserved  uintptr
}

type spDeviceInterfaceData struct {
	CbSize    uint32
	ClassGuid winGUID
	Flags     uint32
	Reserved  uintptr
}

var hidClassGUID = winGUID{Data1: 0x4d1e55b2, Data2: 0xf16f, Data3: 0x11cf, Data4: [8]byte{0x88, 0xcb, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30}}

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
	_             uint16
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

type hidInterfaceMetadata struct {
	Path                string
	VendorID            uint16
	ProductID           uint16
	VersionNumber       uint16
	UsagePage           uint16
	Usage               uint16
	InputReportLength   uint16
	OutputReportLength  uint16
	FeatureReportLength uint16
	Product             string
	Serial              string
	ProbeOK             bool
}

var hidMetadataCache = struct {
	sync.Mutex
	at   map[string]time.Time
	meta map[string]hidInterfaceMetadata
}{at: map[string]time.Time{}, meta: map[string]hidInterfaceMetadata{}}

type rawInputDeviceList struct {
	Device uintptr
	Type   uint32
	_      uint32
}

func utf16Ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func regOpen(root uintptr, path string, access uint32, create bool) (uintptr, error) {
	var h uintptr
	if create {
		var disp uint32
		r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 0, 0, uintptr(access|keyCreateSubKey), 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disp)))
		if r != 0 {
			return 0, syscall.Errno(r)
		}
		return h, nil
	}
	r, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, uintptr(access), uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return h, nil
}

func readRegistryDWORD(root uintptr, path, name string) (uint32, bool) {
	h, err := regOpen(root, path, keyQueryValue, false)
	if err != nil {
		return 0, false
	}
	defer pRegCloseKey.Call(h)
	var typ, size uint32 = 0, 4
	var v uint32
	r, _, _ := pRegQueryValueExW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&size)))
	return v, r == 0 && typ == regDWORD
}

func writeRegistryDWORD(root uintptr, path, name string, value uint32) error {
	h, err := regOpen(root, path, keySetValue, true)
	if err != nil {
		return err
	}
	defer pRegCloseKey.Call(h)
	r, _, _ := pRegSetValueExW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, regDWORD, uintptr(unsafe.Pointer(&value)), 4)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func readRegistryString(root uintptr, path, name string) (string, bool) {
	h, err := regOpen(root, path, keyQueryValue, false)
	if err != nil {
		return "", false
	}
	defer pRegCloseKey.Call(h)
	var typ, size uint32
	r, _, _ := pRegQueryValueExW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if r != 0 || typ != regSZ || size < 2 {
		return "", false
	}
	buf := make([]uint16, size/2+1)
	r, _, _ = pRegQueryValueExW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func writeRegistryString(root uintptr, path, name, value string) error {
	h, err := regOpen(root, path, keySetValue, true)
	if err != nil {
		return err
	}
	defer pRegCloseKey.Call(h)
	u, _ := syscall.UTF16FromString(value)
	r, _, _ := pRegSetValueExW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, regSZ, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func deleteRegistryValue(root uintptr, path, name string) error {
	h, err := regOpen(root, path, keySetValue, false)
	if err != nil {
		return nil
	}
	defer pRegCloseKey.Call(h)
	r, _, _ := pRegDeleteValueW.Call(h, uintptr(unsafe.Pointer(utf16Ptr(name))))
	if r != 0 && r != 2 {
		return syscall.Errno(r)
	}
	return nil
}

func DeleteMachineRegistryTree(path string) error {
	r, _, _ := pRegDeleteTreeW.Call(hkeyLocalMachine, uintptr(unsafe.Pointer(utf16Ptr(path))))
	if r != 0 && r != 2 { // ERROR_FILE_NOT_FOUND is already the desired state.
		return syscall.Errno(r)
	}
	return nil
}

func WindowsAppsUseLightTheme() bool {
	v, ok := readRegistryDWORD(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme")
	return ok && v != 0
}

func SetUserStartup(exe string, enabled bool) error {
	const path = `Software\Microsoft\Windows\CurrentVersion\Run`
	if !enabled {
		return deleteRegistryValue(hkeyCurrentUser, path, "LogiMate")
	}
	return writeRegistryString(hkeyCurrentUser, path, "LogiMate", `"`+exe+`"`)
}

func UserStartupMatches(exe string) bool {
	v, ok := readRegistryString(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Run`, "LogiMate")
	if !ok {
		return false
	}
	v = strings.Trim(strings.TrimSpace(v), `"`)
	return strings.EqualFold(v, exe)
}

func HVCIEnabledNative() bool {
	v, ok := readRegistryDWORD(hkeyLocalMachine, `SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity`, "Enabled")
	return ok && v == 1
}
func IsProcessRunningNative(name string) bool {
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == invalidHandle {
		return false
	}
	defer pCloseHandle.Call(snap)
	pe := processEntry32W{Size: uint32(unsafe.Sizeof(processEntry32W{}))}
	r, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		if strings.EqualFold(syscall.UTF16ToString(pe.ExeFile[:]), name) {
			return true
		}
		r, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return false
}

func ListRunningProcessNamesNative() map[string]bool {
	out := map[string]bool{}
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == invalidHandle {
		return out
	}
	defer pCloseHandle.Call(snap)
	pe := processEntry32W{Size: uint32(unsafe.Sizeof(processEntry32W{}))}
	r, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		name := strings.TrimSpace(syscall.UTF16ToString(pe.ExeFile[:]))
		name = strings.TrimSuffix(name, ".exe")
		if name != "" {
			out[name] = true
		}
		r, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return out
}

func ForegroundProcessNameNative() string {
	hwnd, _, _ := pGetForegroundWindowNative.Call()
	if hwnd == 0 {
		return ""
	}
	var pid uint32
	pGetWindowThreadProcessIdNative.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == invalidHandle {
		return ""
	}
	defer pCloseHandle.Call(snap)
	pe := processEntry32W{Size: uint32(unsafe.Sizeof(processEntry32W{}))}
	r, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		if pe.ProcessID == pid {
			return strings.TrimSuffix(strings.TrimSpace(syscall.UTF16ToString(pe.ExeFile[:])), ".exe")
		}
		r, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return ""
}

func FindProcessExecutableNative(name string) string {
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == invalidHandle {
		return ""
	}
	defer pCloseHandle.Call(snap)
	pe := processEntry32W{Size: uint32(unsafe.Sizeof(processEntry32W{}))}
	r, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		if strings.EqualFold(syscall.UTF16ToString(pe.ExeFile[:]), name) {
			h, _, _ := pOpenProcessNative.Call(processQueryLimitedInformation, 0, uintptr(pe.ProcessID))
			if h != 0 {
				buf := make([]uint16, 32768)
				sz := uint32(len(buf))
				ok, _, _ := pQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)))
				pCloseHandle.Call(h)
				if ok != 0 && sz > 0 {
					return strings.TrimSpace(syscall.UTF16ToString(buf[:sz]))
				}
			}
		}
		r, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return ""
}

func setupPropString(set uintptr, dev *spDevInfoData, prop uint32) string {
	buf := make([]uint16, 512)
	var regType, needed uint32
	r, _, _ := pSetupDiGetDeviceRegistryPropertyW.Call(set, uintptr(unsafe.Pointer(dev)), uintptr(prop), uintptr(unsafe.Pointer(&regType)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2), uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func setupDevicePropertyString(set uintptr, dev *spDevInfoData, key *devPropKey) string {
	if pSetupDiGetDevicePropertyW.Find() != nil {
		return ""
	}
	buf := make([]uint16, 512)
	var typ, needed uint32
	r, _, _ := pSetupDiGetDevicePropertyW.Call(
		set, uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2),
		uintptr(unsafe.Pointer(&needed)), 0,
	)
	if r == 0 || typ != devPropTypeString {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func setupDevicePropertyStringListFirst(set uintptr, dev *spDevInfoData, key *devPropKey) string {
	if pSetupDiGetDevicePropertyW.Find() != nil {
		return ""
	}
	var typ, needed uint32
	// First ask Windows for the exact REG_MULTI_SZ byte count. Failure with a
	// non-zero required size is the normal insufficient-buffer contract.
	pSetupDiGetDevicePropertyW.Call(
		set, uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(&typ)), 0, 0, uintptr(unsafe.Pointer(&needed)), 0,
	)
	if needed < 2 || needed > 64*1024 {
		return ""
	}
	buf := make([]uint16, int(needed/2)+1)
	r, _, _ := pSetupDiGetDevicePropertyW.Call(
		set, uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), 0,
	)
	if r == 0 || typ != devPropTypeStringList {
		return ""
	}
	// LocationPaths is a MULTI_SZ ordered list. The first path is Windows' most
	// specific location and is sufficient for a stable same-port wheel identity.
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func setupDevicePropertyGUID(set uintptr, dev *spDevInfoData, key *devPropKey) string {
	if pSetupDiGetDevicePropertyW.Find() != nil {
		return ""
	}
	var g winGUID
	var typ, needed uint32
	r, _, _ := pSetupDiGetDevicePropertyW.Call(
		set, uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&g)), uintptr(unsafe.Sizeof(g)),
		uintptr(unsafe.Pointer(&needed)), 0,
	)
	if r == 0 || typ != devPropTypeGUID {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3, g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7]))
}

func interfaceDetailCBSize() uint32 {
	// SP_DEVICE_INTERFACE_DETAIL_DATA_W uses cbSize=8 on 64-bit Windows and 6
	// on 32-bit Windows. LogiMate currently ships amd64/arm64, but keep the
	// calculation correct for diagnostic builds as well.
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return 8
	}
	return 6
}

func utf16FromDetailBuffer(buf []byte, offset int) string {
	if offset < 0 || offset >= len(buf) {
		return ""
	}
	vals := make([]uint16, 0, (len(buf)-offset)/2)
	for i := offset; i+1 < len(buf); i += 2 {
		v := uint16(buf[i]) | uint16(buf[i+1])<<8
		if v == 0 {
			break
		}
		vals = append(vals, v)
	}
	return strings.TrimSpace(syscall.UTF16ToString(vals))
}

func hidStringProperty(h syscall.Handle, proc *syscall.LazyProc) string {
	buf := make([]uint16, 256)
	r, _, _ := proc.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2))
	if r == 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func probeHIDInterfaceMetadata(path string) hidInterfaceMetadata {
	meta := hidInterfaceMetadata{Path: path}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return meta
	}
	// Metadata probing intentionally opens with desiredAccess=0 and shared
	// read/write. This mirrors HidSharp-style enumeration: discovery must not
	// steal the wheel from a game, LogiMate's own reader, or the output lease.
	h, err := syscall.CreateFile(p, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil || h == syscall.InvalidHandle {
		return meta
	}
	defer syscall.CloseHandle(h)

	attrs := hiddAttributes{Size: uint32(unsafe.Sizeof(hiddAttributes{}))}
	if r, _, _ := pHidDGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&attrs))); r != 0 {
		meta.VendorID, meta.ProductID, meta.VersionNumber = attrs.VendorID, attrs.ProductID, attrs.VersionNumber
		meta.ProbeOK = true
	}
	var preparsed uintptr
	if r, _, _ := pHidDGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&preparsed))); r != 0 && preparsed != 0 {
		caps := hidpCaps{}
		status, _, _ := pHidPGetCaps.Call(preparsed, uintptr(unsafe.Pointer(&caps)))
		pHidDFreePreparsedData.Call(preparsed)
		if int32(status) >= 0 {
			meta.UsagePage = caps.UsagePage
			meta.Usage = caps.Usage
			meta.InputReportLength = caps.InputReportByteLength
			meta.OutputReportLength = caps.OutputReportByteLength
			meta.FeatureReportLength = caps.FeatureReportByteLength
			meta.ProbeOK = true
		}
	}
	meta.Product = hidStringProperty(h, pHidDGetProductString)
	meta.Serial = hidStringProperty(h, pHidDGetSerialNumber)
	return meta
}

func hidInterfaceMetadataCached(path string, maxAge time.Duration) hidInterfaceMetadata {
	key := strings.ToLower(strings.TrimSpace(path))
	if key == "" {
		return hidInterfaceMetadata{}
	}
	hidMetadataCache.Lock()
	if at := hidMetadataCache.at[key]; !at.IsZero() && time.Since(at) < maxAge {
		meta := hidMetadataCache.meta[key]
		hidMetadataCache.Unlock()
		return meta
	}
	hidMetadataCache.Unlock()
	meta := probeHIDInterfaceMetadata(path)
	hidMetadataCache.Lock()
	hidMetadataCache.at[key] = time.Now()
	hidMetadataCache.meta[key] = meta
	hidMetadataCache.Unlock()
	return meta
}

func invalidateHIDMetadataCache() {
	hidMetadataCache.Lock()
	hidMetadataCache.at = map[string]time.Time{}
	hidMetadataCache.meta = map[string]hidInterfaceMetadata{}
	hidMetadataCache.Unlock()
}

func isSupportedWheelPIDValue(pid uint16) bool {
	switch pid {
	case 0xC294, 0xC299, 0xC29A, 0xC29B:
		return true
	default:
		return false
	}
}

func hidPathPreferenceLess(a, b hidInterfaceMetadata) bool {
	// OpenG27 picks the candidate with the largest output report. Keep that as
	// the primary rule, then prefer a richer input report and joystick/gamepad
	// top-level collection. Stable path order is the final tie breaker.
	if a.OutputReportLength != b.OutputReportLength {
		return a.OutputReportLength > b.OutputReportLength
	}
	if a.InputReportLength != b.InputReportLength {
		return a.InputReportLength > b.InputReportLength
	}
	aGame := a.UsagePage == 0x01 && (a.Usage == 0x04 || a.Usage == 0x05)
	bGame := b.UsagePage == 0x01 && (b.Usage == 0x04 || b.Usage == 0x05)
	if aGame != bGame {
		return aGame
	}
	return strings.ToLower(a.Path) < strings.ToLower(b.Path)
}

func rankHIDPathsOpenG27Style(paths []string, pid string) []string {
	var metas []hidInterfaceMetadata
	seen := map[string]bool{}
	for _, path := range paths {
		if pid != "" && devicePID(path) != pid {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(path))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		m := hidInterfaceMetadataCached(path, 2*time.Second)
		if m.Path == "" {
			m.Path = path
		}
		metas = append(metas, m)
	}
	sort.SliceStable(metas, func(i, j int) bool { return hidPathPreferenceLess(metas[i], metas[j]) })
	out := make([]string, 0, len(metas))
	for _, m := range metas {
		out = append(out, m.Path)
	}
	return out
}

// enumerateHIDLogitechWheelInterfacesNative mirrors the important part of
// HidSharp/OpenG27 discovery: enumerate the actual present HID interfaces,
// then filter by Logitech VID and the classic wheel PIDs. Unlike Raw Input,
// this path does not depend on the device currently producing input reports.
// SetupAPI also gives us the backing devnode, so the result is real PnP
// evidence rather than a synthetic path-only fallback.
func enumerateHIDLogitechWheelInterfacesNative() ([]Device, []string, error) {
	set, _, callErr := pSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&hidClassGUID)), 0, 0, digcfPresent|digcfDeviceInterface,
	)
	if set == 0 || set == invalidHandle {
		return nil, nil, fmt.Errorf("SetupDiGetClassDevsW(HID): %v", callErr)
	}
	defer pSetupDiDestroyDeviceInfoList.Call(set)

	var devices []Device
	var pathMetas []hidInterfaceMetadata
	seenPath := map[string]bool{}
	seenDevice := map[string]bool{}
	for i := uint32(0); ; i++ {
		iface := spDeviceInterfaceData{CbSize: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
		r, _, enumErr := pSetupDiEnumDeviceInterfaces.Call(
			set, 0, uintptr(unsafe.Pointer(&hidClassGUID)), uintptr(i), uintptr(unsafe.Pointer(&iface)),
		)
		if r == 0 {
			if errno, ok := enumErr.(syscall.Errno); ok && errno == 259 { // ERROR_NO_MORE_ITEMS
				break
			}
			return nil, nil, fmt.Errorf("SetupDiEnumDeviceInterfaces index %d: %v", i, enumErr)
		}

		var needed uint32
		dev := spDevInfoData{CbSize: uint32(unsafe.Sizeof(spDevInfoData{}))}
		// First call obtains the exact detail size. ERROR_INSUFFICIENT_BUFFER is
		// expected and proves the interface exists.
		pSetupDiGetDeviceInterfaceDetailW.Call(
			set, uintptr(unsafe.Pointer(&iface)), 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&dev)),
		)
		if needed < 8 || needed > 64*1024 {
			continue
		}
		detail := make([]byte, needed)
		cb := interfaceDetailCBSize()
		detail[0] = byte(cb)
		detail[1] = byte(cb >> 8)
		detail[2] = byte(cb >> 16)
		detail[3] = byte(cb >> 24)
		dev = spDevInfoData{CbSize: uint32(unsafe.Sizeof(spDevInfoData{}))}
		r, _, detailErr := pSetupDiGetDeviceInterfaceDetailW.Call(
			set, uintptr(unsafe.Pointer(&iface)), uintptr(unsafe.Pointer(&detail[0])), uintptr(needed),
			uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&dev)),
		)
		if r == 0 {
			return nil, nil, fmt.Errorf("SetupDiGetDeviceInterfaceDetailW index %d: %v", i, detailErr)
		}
		path := utf16FromDetailBuffer(detail, 4)
		meta := hidInterfaceMetadataCached(path, 2*time.Second)
		pathClaimsWheel := supportedWheelPIDText(strings.ToUpper(path))
		attrsClaimWheel := meta.ProbeOK && meta.VendorID == 0x046D && isSupportedWheelPIDValue(meta.ProductID)
		if !pathClaimsWheel && !attrsClaimWheel {
			continue
		}
		pathKey := strings.ToLower(path)
		if path != "" && !seenPath[pathKey] {
			seenPath[pathKey] = true
			if meta.Path == "" {
				meta.Path = path
			}
			pathMetas = append(pathMetas, meta)
		}

		idBuf := make([]uint16, 1024)
		var idNeeded uint32
		r, _, _ = pSetupDiGetDeviceInstanceIdW.Call(
			set, uintptr(unsafe.Pointer(&dev)), uintptr(unsafe.Pointer(&idBuf[0])), uintptr(len(idBuf)), uintptr(unsafe.Pointer(&idNeeded)),
		)
		if r == 0 {
			continue
		}
		id := strings.TrimSpace(syscall.UTF16ToString(idBuf))
		if !supportedWheelPIDText(strings.ToUpper(id)) {
			// HidSharp/OpenG27 can still see a device even if one Windows devnode
			// string is unusual. Attributes prove the HID interface, but destructive
			// operations still require a correlatable supported PnP instance.
			continue
		}
		key := strings.ToLower(id)
		if seenDevice[key] {
			continue
		}
		seenDevice[key] = true
		name := setupPropString(set, &dev, spdrpFriendlyName)
		if name == "" {
			name = setupPropString(set, &dev, spdrpDeviceDesc)
		}
		service := setupPropString(set, &dev, spdrpService)
		driver := setupDevicePropertyString(set, &dev, &devpkeyDeviceDriverInfPath)
		if driver == "" {
			driver = setupPropString(set, &dev, spdrpDriver)
		}
		containerID := setupDevicePropertyGUID(set, &dev, &devpkeyDeviceContainerID)
		parentID := setupDevicePropertyString(set, &dev, &devpkeyDeviceParent)
		locationPath := setupDevicePropertyStringListFirst(set, &dev, &devpkeyDeviceLocationPaths)
		stableID := DeriveStableWheelIDWithLocation(id, parentID, locationPath)
		physicalID := id
		if containerID != "" {
			physicalID = "container:" + containerID
		} else if stableID != "" {
			physicalID = stableID
		} else if supportedWheelPIDText(parentID) {
			physicalID = parentID
		}
		modelEvidenceName := strings.TrimSpace(strings.TrimSpace(name) + " " + strings.TrimSpace(meta.Product))
		devices = append(devices, Device{
			Status: "OK", Name: name, Service: service, INF: driver, InstanceID: id,
			ParentID: parentID, PhysicalID: physicalID, ContainerID: containerID,
			LocationPath: locationPath, StableID: stableID, Model: IdentifyWheelModel(modelEvidenceName, id),
			HIDProduct: meta.Product, HIDSerial: meta.Serial,
		})
	}
	sort.SliceStable(pathMetas, func(i, j int) bool { return hidPathPreferenceLess(pathMetas[i], pathMetas[j]) })
	paths := make([]string, 0, len(pathMetas))
	for _, meta := range pathMetas {
		paths = append(paths, meta.Path)
	}
	return devices, paths, nil
}

func mergeDeviceEvidence(primary, secondary []Device) []Device {
	// Device-tree enumeration and HID-interface enumeration frequently describe
	// the exact same Windows devnode. The HID pass, however, carries evidence
	// that the all-class PnP pass cannot provide (product string, serial, and in
	// future HID-specific metadata). D5.2 originally treated an equal InstanceID
	// as a plain duplicate and discarded the secondary record. That made the
	// Direct-HID diagnostics show "G27 Racing Wheel" while the automatic C294
	// model-confirmation path only saw the generic PnP record and therefore
	// waited for a manual confirmation. Merge duplicate records field-by-field
	// instead: topology/driver identity remains authoritative from primary, while
	// stronger non-empty HID evidence enriches it.
	out := append([]Device(nil), primary...)
	byID := make(map[string]int, len(out))
	for i, d := range out {
		if id := strings.ToLower(strings.TrimSpace(d.InstanceID)); id != "" {
			byID[id] = i
		}
	}
	for _, d := range secondary {
		id := strings.ToLower(strings.TrimSpace(d.InstanceID))
		if id == "" {
			continue
		}
		if i, ok := byID[id]; ok {
			p := &out[i]
			if strings.TrimSpace(p.HIDProduct) == "" && strings.TrimSpace(d.HIDProduct) != "" {
				p.HIDProduct = d.HIDProduct
			}
			if strings.TrimSpace(p.HIDSerial) == "" && strings.TrimSpace(d.HIDSerial) != "" {
				p.HIDSerial = d.HIDSerial
			}
			if strings.TrimSpace(p.ParentID) == "" && strings.TrimSpace(d.ParentID) != "" {
				p.ParentID = d.ParentID
			}
			if strings.TrimSpace(p.ContainerID) == "" && strings.TrimSpace(d.ContainerID) != "" {
				p.ContainerID = d.ContainerID
			}
			if strings.TrimSpace(p.LocationPath) == "" && strings.TrimSpace(d.LocationPath) != "" {
				p.LocationPath = d.LocationPath
			}
			if strings.TrimSpace(p.StableID) == "" && strings.TrimSpace(d.StableID) != "" {
				p.StableID = d.StableID
			}
			if strings.TrimSpace(p.PhysicalID) == "" && strings.TrimSpace(d.PhysicalID) != "" {
				p.PhysicalID = d.PhysicalID
			}
			// A direct HID product string is more model-specific than a generic
			// "Driving Force USB"/C294 PnP label. Preserve an already-native
			// authoritative model, otherwise allow the HID-derived model to enrich
			// the logical evidence used by current-session consensus.
			if (p.Model == "" || IsCompatibilityModel(p.Model)) && IsSupportedWheelModel(d.Model) {
				p.Model = d.Model
			}
			continue
		}
		byID[id] = len(out)
		out = append(out, d)
	}
	return out
}

func mergeHIDPaths(groups ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range groups {
		for _, path := range group {
			v := strings.TrimSpace(path)
			if v == "" {
				continue
			}
			k := strings.ToLower(v)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func detectDevicesNative() ([]Device, error) {
	set, _, e := pSetupDiGetClassDevsW.Call(0, 0, 0, digcfPresent|digcfAllClasses)
	if set == 0 || set == invalidHandle {
		return nil, fmt.Errorf("SetupDiGetClassDevsW: %v", e)
	}
	defer pSetupDiDestroyDeviceInfoList.Call(set)
	var out []Device
	for i := uint32(0); ; i++ {
		d := spDevInfoData{CbSize: uint32(unsafe.Sizeof(spDevInfoData{}))}
		r, _, err := pSetupDiEnumDeviceInfo.Call(set, uintptr(i), uintptr(unsafe.Pointer(&d)))
		if r == 0 {
			if errno, ok := err.(syscall.Errno); ok && errno == 259 { // ERROR_NO_MORE_ITEMS
				break
			}
			// Never return a partial topology as a successful scan. Safety-critical
			// selection/output code relies on the list being complete.
			return nil, fmt.Errorf("SetupDiEnumDeviceInfo index %d: %v", i, err)
		}
		idBuf := make([]uint16, 1024)
		var needed uint32
		r, _, _ = pSetupDiGetDeviceInstanceIdW.Call(set, uintptr(unsafe.Pointer(&d)), uintptr(unsafe.Pointer(&idBuf[0])), uintptr(len(idBuf)), uintptr(unsafe.Pointer(&needed)))
		if r == 0 {
			continue
		}
		id := syscall.UTF16ToString(idBuf)
		u := strings.ToUpper(id)
		if !supportedWheelPIDText(u) {
			continue
		}
		name := setupPropString(set, &d, spdrpFriendlyName)
		if name == "" {
			name = setupPropString(set, &d, spdrpDeviceDesc)
		}
		service := setupPropString(set, &d, spdrpService)
		driver := setupDevicePropertyString(set, &d, &devpkeyDeviceDriverInfPath)
		if driver == "" {
			// SPDRP_DRIVER is a driver software-key name rather than an INF path,
			// but it is still useful diagnostic evidence on unusual images.
			driver = setupPropString(set, &d, spdrpDriver)
		}
		containerID := setupDevicePropertyGUID(set, &d, &devpkeyDeviceContainerID)
		parentID := setupDevicePropertyString(set, &d, &devpkeyDeviceParent)
		locationPath := setupDevicePropertyStringListFirst(set, &d, &devpkeyDeviceLocationPaths)
		stableID := DeriveStableWheelIDWithLocation(id, parentID, locationPath)
		physicalID := id
		if containerID != "" {
			physicalID = "container:" + containerID
		} else if stableID != "" {
			physicalID = stableID
		} else if supportedWheelPIDText(parentID) {
			physicalID = parentID
		}
		out = append(out, Device{Status: "OK", Name: name, Service: service, INF: driver, InstanceID: id, ParentID: parentID, PhysicalID: physicalID, ContainerID: containerID, LocationPath: locationPath, StableID: stableID, Model: IdentifyWheelModel(name, id)})
	}
	return out, nil
}

func EnumerateRawInputLogitechWheelsStrict() ([]string, error) {
	var count uint32
	cb := uint32(unsafe.Sizeof(rawInputDeviceList{}))
	r, _, callErr := pGetRawInputDeviceList.Call(0, uintptr(unsafe.Pointer(&count)), uintptr(cb))
	if int32(r) == -1 {
		return nil, fmt.Errorf("GetRawInputDeviceList(count): %v", callErr)
	}
	if count == 0 {
		return nil, nil
	}
	list := make([]rawInputDeviceList, count)
	r, _, callErr = pGetRawInputDeviceList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(unsafe.Pointer(&count)), uintptr(cb))
	if int32(r) == -1 {
		return nil, fmt.Errorf("GetRawInputDeviceList(data): %v", callErr)
	}
	var out []string
	for _, d := range list[:count] {
		if d.Type != rimTypeHID {
			continue
		}
		var chars uint32
		rr, _, nameErr := pGetRawInputDeviceInfoW.Call(d.Device, ridiDeviceName, 0, uintptr(unsafe.Pointer(&chars)))
		if int32(rr) == -1 {
			return nil, fmt.Errorf("GetRawInputDeviceInfoW(size): %v", nameErr)
		}
		if chars == 0 {
			continue
		}
		buf := make([]uint16, chars+1)
		rr, _, nameErr = pGetRawInputDeviceInfoW.Call(d.Device, ridiDeviceName, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&chars)))
		if int32(rr) == -1 {
			return nil, fmt.Errorf("GetRawInputDeviceInfoW(name): %v", nameErr)
		}
		name := syscall.UTF16ToString(buf)
		if supportedWheelPIDText(strings.ToUpper(name)) {
			out = append(out, name)
		}
	}
	return out, nil
}

// discoverLogitechHIDPathsOpenG27Style performs direct HID-interface discovery
// first (matching OpenG27/HidSharp's strongest behavior) and uses Raw Input only
// as a supplemental source. A failure in one source does not hide a wheel found
// by the other source.
// DirectHIDCandidateDiagnostics returns a human-readable snapshot of the direct
// HID candidates LogiMate sees. It is intentionally read-only and mirrors the
// practical rescue visibility OpenG27 exposes through its all-HID device list.
func DirectHIDCandidateDiagnostics(paths []string) string {
	if len(paths) == 0 {
		if discovered, _ := discoverLogitechHIDPathsOpenG27Style(); len(discovered) > 0 {
			paths = discovered
		}
	}
	if len(paths) == 0 {
		return "Keine unterstützte Logitech-HID-Schnittstelle (046D / C294,C299,C29A,C29B) direkt sichtbar."
	}
	paths = rankHIDPathsOpenG27Style(paths, "")
	var b strings.Builder
	phase, detail, running := NativeActivationStatus()
	if phase != "" || detail != "" || running {
		fmt.Fprintf(&b, "Native-Aktivierung: Phase=%s · läuft=%v\r\n%s\r\n\r\n", phase, running, detail)
	}
	for i, path := range paths {
		m := hidInterfaceMetadataCached(path, 2*time.Second)
		pid := devicePID(path)
		if m.ProbeOK && m.ProductID != 0 {
			pid = strings.ToUpper(fmt.Sprintf("%04X", m.ProductID))
		}
		product := strings.TrimSpace(m.Product)
		if product == "" {
			product = "(kein HID-Produktname)"
		}
		fmt.Fprintf(&b, "%d. PID %s · %s · In=%d Out=%d Feature=%d · Usage=%04X/%04X\r\n%s\r\n",
			i+1, pid, product, m.InputReportLength, m.OutputReportLength, m.FeatureReportLength, m.UsagePage, m.Usage, path)
	}
	return strings.TrimSpace(b.String())
}

func discoverLogitechHIDPathsOpenG27Style() ([]string, error) {
	_, hidPaths, hidErr := enumerateHIDLogitechWheelInterfacesNative()
	rawPaths, rawErr := EnumerateRawInputLogitechWheelsStrict()
	paths := mergeHIDPaths(hidPaths, rawPaths)
	if len(paths) > 0 {
		return paths, nil
	}
	if hidErr != nil && rawErr != nil {
		return nil, fmt.Errorf("direkte HID-Erkennung: %v; Raw Input: %v", hidErr, rawErr)
	}
	if hidErr != nil {
		return nil, fmt.Errorf("direkte HID-Erkennung: %w", hidErr)
	}
	if rawErr != nil {
		return nil, fmt.Errorf("Raw Input: %w", rawErr)
	}
	return nil, nil
}

// EnumerateRawInputLogitechWheels is retained for non-critical callers. State
// collection uses the strict variant so discovery failure is never confused
// with a complete "no HID devices" result.
func EnumerateRawInputLogitechWheels() []string {
	out, _ := EnumerateRawInputLogitechWheelsStrict()
	return out
}

func NativeOSString() string {
	const path = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	product, _ := readRegistryString(hkeyLocalMachine, path, "ProductName")
	display, _ := readRegistryString(hkeyLocalMachine, path, "DisplayVersion")
	build, _ := readRegistryString(hkeyLocalMachine, path, "CurrentBuildNumber")
	if n, err := strconv.Atoi(strings.TrimSpace(build)); err == nil && n >= 22000 {
		product = "Windows 11"
	}
	if strings.TrimSpace(product) == "" {
		product = "Windows"
	}
	parts := []string{product}
	if display != "" {
		parts = append(parts, display)
	}
	if build != "" {
		parts = append(parts, "build "+build)
	}
	return strings.Join(parts, " ")
}
