package tuntap

import (
	"encoding/binary"
	"errors"
	"fmt"
	w "golang.org/x/sys/windows"
	r "golang.org/x/sys/windows/registry"
	"os/exec"
	"regexp"
	"syscall"
	"unsafe"
)

func CtlCode(DeviceType uint32, Function uint32, Method uint32, Access uint32) uint32 {
	return ((DeviceType) << 16) | ((Access) << 14) | ((Function) << 2) | (Method)
}

func TapControlCode(request uint32, method uint32) uint32 {
	var FileDeviceUnknown, FileAnyAccess uint32
	FileDeviceUnknown = 0x00000022
	FileAnyAccess = 0
	return CtlCode(FileDeviceUnknown, request, method, FileAnyAccess)
}

var MethodBuffered uint32 = 0
var TapIoctlGetMac = TapControlCode(1, MethodBuffered)

//var TAP_IOCTL_GET_VERSION = TAP_CONTROL_CODE(2, METHOD_BUFFERED)
//var TAP_IOCTL_GET_MTU = TAP_CONTROL_CODE(3, METHOD_BUFFERED)
//var TAP_IOCTL_GET_INFO = TAP_CONTROL_CODE(4, METHOD_BUFFERED)
//var TAP_IOCTL_CONFIG_POINT_TO_POINT = TAP_CONTROL_CODE(5, METHOD_BUFFERED)
var TapIoctlSetMediaStatus = TapControlCode(6, MethodBuffered)

//var TAP_IOCTL_CONFIG_DHCP_MASQ = TAP_CONTROL_CODE(7, METHOD_BUFFERED)
//var TAP_IOCTL_GET_LOG_LINE = TAP_CONTROL_CODE(8, METHOD_BUFFERED)
//var TAP_IOCTL_CONFIG_DHCP_SET_OPT = TAP_CONTROL_CODE(9, METHOD_BUFFERED)

const (
	// tapDriverKey is the location of the TAP driver key.
	tapDriverKey = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	// netConfigKey is the location of the TAP adapter's network config.
	netConfigKey = `SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}`
)

var (
	nCreateEvent,
	nResetEvent,
	nGetOverlappedResult uintptr
)

type TapWindows struct {
	handler        w.Handle
	ro             *syscall.Overlapped
	wo             *syscall.Overlapped
	devName        string
	deviceRegistry string
}

func init() {
	k32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		panic("LoadLibrary " + err.Error())
	}
	defer func() {
		_ = syscall.FreeLibrary(k32)
	}()

	nCreateEvent = getProcAddr(k32, "CreateEventW")
	nResetEvent = getProcAddr(k32, "ResetEvent")
	nGetOverlappedResult = getProcAddr(k32, "GetOverlappedResult")
}

func getProcAddr(lib syscall.Handle, name string) uintptr {
	addr, err := syscall.GetProcAddress(lib, name)
	if err != nil {
		panic(name + " " + err.Error())
	}
	return addr
}

func lookupAdapterRegPath(deviceRegistry string) string {
	key, ok := r.OpenKey(r.LOCAL_MACHINE, tapDriverKey, r.READ)
	if ok != nil {
		println(ok.Error())
		return ""
	}
	keys, ok := key.ReadSubKeyNames(0)
	_ = key.Close()
	for _, subkey := range keys {
		keypath := tapDriverKey + "\\" + subkey
		key, ok = r.OpenKey(r.LOCAL_MACHINE, keypath, r.READ)
		if ok != nil {
			continue
		}
		devRegID, _, _ := key.GetStringValue("NetCfgInstanceId")
		if devRegID == deviceRegistry {
			return keypath
		}
		_ = key.Close()
	}
	return ""
}

func CreateTapDevice(name string) (TapDevice, error) {
	name = ""
	key, ok := r.OpenKey(r.LOCAL_MACHINE, netConfigKey, r.READ)
	if ok != nil {
		return nil, errors.New("没有找到虚拟网卡信息,请确认已经安装TapAdapterV9")
	}
	keys, ok := key.ReadSubKeyNames(0)
	_ = key.Close()
	tapWindows := &TapWindows{}
	for _, subKey := range keys {
		tapWindows.deviceRegistry = subKey
		keyPath := netConfigKey + "\\" + subKey + "\\Connection"
		key, ok = r.OpenKey(r.LOCAL_MACHINE, keyPath, r.READ)
		if ok != nil {
			continue
		}
		tapWindows.devName, _, _ = key.GetStringValue("Name")
		if len(name) > 0 && tapWindows.devName != name {
			continue
		}
		tapName := "\\\\.\\Global\\" + subKey + ".tap"
		filepath := w.StringToUTF16Ptr(tapName)

		tapWindows.handler, ok = w.CreateFile(filepath, w.GENERIC_WRITE|w.GENERIC_READ, 0, nil, w.OPEN_EXISTING, w.FILE_ATTRIBUTE_SYSTEM|syscall.FILE_FLAG_OVERLAPPED, 0)
		if ok == nil {
			_ = tapWindows.setStatus(syscall.Handle(tapWindows.handler), true)
			_ = key.Close()
			return tapWindows, tapWindows.createRWOverlapped()
		} else {
		}
	}
	return nil, errors.New("虚拟网卡启动失败")
}
func (t *TapWindows) Name() string {
	return t.devName
}
func (t *TapWindows) setStatus(fd syscall.Handle, status bool) error {
	var bytesReturned uint32
	buf := make([]byte, syscall.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	code := []byte{0x00, 0x00, 0x00, 0x00}
	if status {
		code[0] = 0x01
	}
	return syscall.DeviceIoControl(fd, TapIoctlSetMediaStatus, &code[0], uint32(4), &buf[0], uint32(len(buf)), &bytesReturned, nil)
}

func (t *TapWindows) createRWOverlapped() error {
	var ro syscall.Overlapped
	r1, _, err := syscall.SyscallN(nCreateEvent, 0, 1, 0, 0)
	if r1 == 0 {
		return err
	}
	ro.HEvent = syscall.Handle(r1)
	t.ro = &ro

	var wo syscall.Overlapped
	r2, _, err := syscall.SyscallN(nCreateEvent, 0, 1, 0, 0)
	if r2 == 0 {
		return err
	}
	wo.HEvent = syscall.Handle(r2)
	t.wo = &wo
	return nil
}
func resetEvent(h syscall.Handle) error {
	r1, _, err := syscall.SyscallN(nResetEvent, uintptr(h))
	if r1 == 0 {
		return err
	}
	return nil
}

func getOverlappedResult(h w.Handle, overlapped *syscall.Overlapped) (int, error) {
	var n int
	r1, _, err := syscall.SyscallN(nGetOverlappedResult, uintptr(h), uintptr(unsafe.Pointer(overlapped)), uintptr(unsafe.Pointer(&n)), 1)
	if r1 == 0 {
		return n, err
	}
	return n, nil
}

func (t *TapWindows) SetMac(mac uint64) error {
	macBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(macBuf, mac)
	macStr := fmt.Sprintf("%02X%02X%02X%02X%02X%02X", macBuf[0], macBuf[1], macBuf[2], macBuf[3], macBuf[4], macBuf[5])

	path := lookupAdapterRegPath(t.deviceRegistry)

	cmd := exec.Command("reg", "add", "HKEY_LOCAL_MACHINE\\"+path, "/v", "MAC", "/d", macStr, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err := cmd.Output()

	cmd = exec.Command("reg", "add", "HKEY_LOCAL_MACHINE\\"+path, "/v", "NetworkAddress", "/d", macStr, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err = cmd.Output()

	_ = w.Close(t.handler)

	cmd = exec.Command("netsh", "interface", "set", "interface", t.devName, "disabled")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err = cmd.Output()

	cmd = exec.Command("netsh", "interface", "set", "interface", t.devName, "enabled")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err = cmd.Output()

	if err != nil {
		return err
	}
	tapName := "\\\\.\\Global\\" + t.deviceRegistry + ".tap"
	filepath := w.StringToUTF16Ptr(tapName)
	var ok error
	t.handler, ok = w.CreateFile(filepath, w.GENERIC_WRITE|w.GENERIC_READ, 0, nil, w.OPEN_EXISTING, w.FILE_ATTRIBUTE_SYSTEM|syscall.FILE_FLAG_OVERLAPPED, 0)
	if ok != nil {
		return ok
	}
	return t.setStatus(syscall.Handle(t.handler), true)
}

func (t *TapWindows) GetMac() (uint64, error) {
	var ret uint32 = 0
	macBuf := make([]byte, 6)
	err := w.DeviceIoControl(t.handler, TapIoctlGetMac,
		&macBuf[0], 6,
		&macBuf[0], 6, &ret, nil)
	if err != nil {
		return 0, err
	}
	data := make([]byte, 8)
	copy(data, macBuf)
	return binary.LittleEndian.Uint64(data), nil
}

func (t *TapWindows) SetIpAddr(addr, mask string) error {
	args := []string{
		"interface", "ipv4", "set", "address",
		t.devName, "static", addr, mask,
	}
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err := cmd.CombinedOutput()
	return err
}

func (t *TapWindows) GetIpAddr() (string, string, error) {
	args := []string{
		"interface", "ipv4", "show", "address",
		t.devName,
	}
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	ipregex, err1 := regexp.Compile("[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+\\s")
	if err1 != nil {
		return "", "", err1
	}
	ipres := ipregex.FindString(string(output))
	maskregex, err2 := regexp.Compile("[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+\\)")
	if err2 != nil {
		return "", "", err2
	}
	maskres := maskregex.FindString(string(output))
	if len(ipres) == 0 || len(maskres) == 0 {
		return "", "", errors.New("没有找到虚拟网卡IP")
	}
	return ipres[0 : len(ipres)-1], maskres[0 : len(maskres)-1], err
}

func (t *TapWindows) SetMtu(mtu uint) error {
	args := []string{
		"interface", "ipv4", "set", "subinterface",
		"name=" + t.devName, fmt.Sprintf("mtu=%d", mtu),
	}
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err := cmd.CombinedOutput()
	return err
}

func (t *TapWindows) Up() error {
	if t.devName == "" {
		return errors.New("device name is empty")
	}
	args := []string{
		"interface", "set", "interface",
		"name=" + t.devName, "admin=ENABLED",
	}
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err := cmd.CombinedOutput()
	return err
}

func (t *TapWindows) Down() error {
	if t.devName == "" {
		return errors.New("device name is empty")
	}
	args := []string{
		"interface", "set", "interface",
		"name=" + t.devName, "admin=DISABLED",
	}
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_, err := cmd.CombinedOutput()
	return err
}

func (t *TapWindows) Close() error {
	_ = syscall.Close(syscall.Handle(t.handler))
	_ = t.setStatus(syscall.Handle(t.handler), false)
	_ = t.Down()
	_ = t.Up()
	return nil
}

func (t *TapWindows) Read(buf []byte) (int, error) {
	if err := resetEvent(t.ro.HEvent); err != nil {
		return 0, err
	}
	var done uint32
	err := syscall.ReadFile(syscall.Handle(t.handler), buf, &done, t.ro)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(done), err
	}
	return getOverlappedResult(t.handler, t.ro)
}

func (t *TapWindows) Write(buf []byte) (int, error) {
	if err := resetEvent(t.wo.HEvent); err != nil {
		return 0, err
	}
	var n uint32
	err := syscall.WriteFile(syscall.Handle(t.handler), buf, &n, t.wo)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(n), err
	}
	return getOverlappedResult(t.handler, t.wo)
}

// route add 172.28.96.0 mask 255.255.224.0 192.168.1.151
func AddRoute(dst, mask, gw string) ([]byte, error) {
	args := []string{
		"add", dst, "mask", mask, gw,
	}
	cmd := exec.Command("route", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd.CombinedOutput()
}

// route delete 172.28.96.0 mask 255.255.224.0 192.168.1.151
func DelRoute(dst, mask, gw string) ([]byte, error) {
	args := []string{
		"delete", dst, "mask", mask, gw,
	}
	cmd := exec.Command("route", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd.CombinedOutput()
}
