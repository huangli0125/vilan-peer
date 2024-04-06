package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var osType = runtime.GOOS

// 根据网卡接口 Index 判断网卡类型 0 unknown 1 RJ45 2 WIFI 3 GPRS
func GetIfType(ifIndex int) (uint32, error) {
	aas, err := adapterAddresses()
	if err != nil {
		return 0, err
	}
	for _, aa := range aas {
		index := aa.IfIndex
		if ifIndex == int(index) {
			switch aa.IfType {
			case windows.IF_TYPE_ETHERNET_CSMACD:
				return 1, nil
			case windows.IF_TYPE_IEEE80211:
				return 2, nil
			case windows.IF_TYPE_PPP:
				return 3, nil
			case windows.IF_TYPE_ATM:
				return 3, nil
			default:
				return 0, nil
			}
		}
	}
	return 0, nil
}
func adapterAddresses() ([]*windows.IpAdapterAddresses, error) {
	var b []byte
	l := uint32(15000) // recommended initial size
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return nil, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
	}
	var aas []*windows.IpAdapterAddresses
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		aas = append(aas, aa)
	}
	return aas, nil
}

func GetLocalIP(ipAddr string) (ip *net.IPNet) {
	faces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	localAddr := ""
	if ipAddr != "" {
		localAddr = ipAddr
	} else {
		dial := net.Dialer{Timeout: time.Second}
		conn, err := dial.Dial("udp", "114.114.114.114:80")
		if err == nil {
			localAddr = strings.Split(conn.LocalAddr().String(), ":")[0]
			_ = conn.Close()
		}
	}

	ip = nil
	for _, i := range faces {
		address, err := i.Addrs()
		if err != nil {
			continue
		}
		if i.Name == "br-lan" { // openwrt
			for _, addr := range address {
				if ip, ok := addr.(*net.IPNet); ok {
					if ip.IP.To4() != nil {
						return ip
					}
				}
			}
		}

		for _, addr := range address {
			// check the address type and if it is not a loopback the display it
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
				if ip.IP.To4() != nil {
					if !strings.Contains(ip.IP.String(), localAddr) {
						continue
					}
					return ip
				}
			}
		}
	}
	return ip
}
func ExtractAddrFromStr(addr string) (sockIp, port uint32, err error) {
	sockStr := strings.Split(addr, ":")
	if len(sockStr) != 2 {
		return 0, 0, errors.New("cannot extract addr")
	}

	if ip, e0 := IpV4toUint32(sockStr[0]); e0 != nil {
		return 0, 0, errors.New("cannot extract ip")
	} else {
		sockIp = ip
	}
	p, e1 := strconv.ParseInt(sockStr[1], 10, 32)
	if e1 != nil {
		return 0, 0, errors.New("cannot extract port")
	}
	port = uint32(p)

	if !IsLittleEndian() {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, sockIp)
		sockIp = binary.BigEndian.Uint32(buf)
	}
	return
}
func IsLittleEndian() bool {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	return *pb == 0x04
}
func IpV4toUint32(ip string) (uint32, error) {
	if len(ip) == 0 {
		return 0, errors.New("err ip string")
	}
	temp := strings.Split(ip, ".")
	if len(temp) != 4 {
		return 0, errors.New("invalid ip string")
	}
	IP := uint32(0)
	for i := 0; i < 4; i++ {
		if val, e := strconv.ParseInt(temp[i], 10, 16); e != nil {
			return 0, e
		} else {
			IP = (IP << 8) | uint32(val)
		}
	}
	return IP, nil
}

func Uint32toIpV4(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}
func MaskBitLen(ipMask uint32) uint32 {
	i := uint32(0)
	mask := uint32(0x80000000)
	for i < 32 {
		if ipMask&mask != 0 {
			i++
			mask = mask >> 1
		} else {
			return i
		}
	}
	return 32
}
func Uint64ToMacStr(mac uint64) string {
	macBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(macBuf, mac)
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", macBuf[0], macBuf[1], macBuf[2], macBuf[3], macBuf[4], macBuf[5])
}

func MacStrToUint64(macStr string) (uint64, error) {
	macBuf := make([]byte, 8)
	n, err := fmt.Sscanf(macStr, "%x:%x:%x:%x:%x:%x", &macBuf[0], &macBuf[1], &macBuf[2], &macBuf[3], &macBuf[4], &macBuf[5])
	if err != nil || n != 6 {
		return 0, errors.New("MAC转换失败")
	}
	return binary.LittleEndian.Uint64(macBuf), nil
}

var MacBroadcast uint64 = 0xFFFFFFFFFFFF //[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
var MacMulticast uint64 = 0x01005E       //[]byte{0x01, 0x00, 0x5E}

// 单播
func IsUniCast(mac uint64) bool {
	return mac != MacBroadcast && mac != MacMulticast
}

func IsBroadcastMac(mac uint64) bool {
	return mac == MacBroadcast
}
func IsMulticastMac(mac uint64) bool {
	return mac == MacMulticast
}

func GetProjectPath() string {
	var projectPath string
	dir, err := filepath.Abs(filepath.Dir(os.Args[0])) // 文件路径
	if err != nil {
		projectPath, _ = os.Getwd() // 启动路径
	} else {
		projectPath = dir
	}
	return projectPath
}

func GetConfigPath() string {
	path := GetProjectPath()
	if osType == "windows" {
		return path + "\\" + "config\\"
	} else {
		return path + "/" + "config/"
	}
}

func GetLogPath() string {
	path := GetProjectPath()
	if osType == "windows" {
		return path + "\\log\\"
	} else {
		return path + "/log/"
	}

}

func GetSystemVersion() uint32 {
	if osType == "windows" {
		version, e := syscall.GetVersion()
		if e != nil {
			version = 7
		} else {
			version = uint32(byte(version) + uint8(version>>8))
		}
		return version
	} else {
		return 10
	}
}
func ClearMap(m *sync.Map) {
	if m == nil {
		return
	}
	keys := make([]interface{}, 0)
	m.Range(func(key, _ interface{}) bool {
		keys = append(keys, key)
		return true
	})
	for i := range keys {
		m.Delete(keys[i])
	}
}
func MapLength(m *sync.Map) int {
	if m == nil {
		return 0
	}
	count := 0
	m.Range(func(key, v interface{}) bool {
		count++
		return true
	})
	return count
}
