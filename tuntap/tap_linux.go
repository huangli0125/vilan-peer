package tuntap

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <net/if.h>
#include <net/if_arp.h>

unsigned char mac[6] = {0};
int getPermMac(char *ifName)
{
	if((0 != getuid()) && (0 != geteuid())){
		return -1;
	}
    struct ifreq ifreq;
    int sock = 0;
    sock = socket(AF_INET,SOCK_STREAM,0);
    if(sock < 0) {
        return -2;
    }
    strcpy(ifreq.ifr_name,ifName);
    if(ioctl(sock,SIOCGIFHWADDR,&ifreq) < 0) {
        return -3;
    }
	close(sock);
    int i = 0;
    for(i = 0; i < 6; i++){
        mac[i] = (unsigned char)ifreq.ifr_hwaddr.sa_data[i];
    }
    return 0;
}
int setPermMac(char *ifname,unsigned char *mac)
{
	if((0 != getuid()) && (0 != geteuid())){
		return -1;
	}
 	struct ifreq ifr;

	int fd=-1;
 	if ((fd = socket(AF_INET, SOCK_STREAM, 0)) < 0)
	{
		return -2;
	}
    memset(&ifr, 0, sizeof(ifr));
    strncpy(ifr.ifr_name, ifname, IFNAMSIZ);
    ifr.ifr_name[IFNAMSIZ-1] = '\0';

    ifr.ifr_hwaddr.sa_family = ARPHRD_ETHER;
    memcpy(ifr.ifr_hwaddr.sa_data, mac, 6);

    if(ioctl(fd, SIOCSIFHWADDR, &ifr) == -1) {
        return -3;
    }
	close(fd);
	return 0;
}
*/
import "C"
import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

type TapLinux struct {
	fd int
	io.ReadWriteCloser
	devName string
}

const (
	cIFFTUN        = 0x0001
	cIFFTAP        = 0x0002
	cIFFNOPI       = 0x1000
	cIFFMULTIQUEUE = 0x0100
)

type ifReq struct {
	Name  [0x10]byte
	Flags uint16
	pad   [0x28 - 0x10 - 2]byte
}

func ioctl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(request), argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func CreateTapDevice(name string) (TapDevice, error) {
	if len(name) == 0 {
		return nil, errors.New("没有有效的名称")
	}

	var tapLinux TapLinux
	tapLinux.devName = name
	var err error
	tapLinux.fd, err = syscall.Open(
		"/dev/net/tun", os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	var req ifReq
	req.Flags = cIFFNOPI | cIFFTAP | cIFFMULTIQUEUE
	copy(req.Name[:0x0f], name)

	err = ioctl(uintptr(tapLinux.fd), syscall.TUNSETIFF, uintptr(unsafe.Pointer(&req)))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("虚拟网卡创建失败:%s", err.Error()))
	}
	tapLinux.ReadWriteCloser = os.NewFile(uintptr(tapLinux.fd), "tun")
	return &tapLinux, nil
}

func (t *TapLinux) Name() string {
	return t.devName
}
func (t *TapLinux) SetMac(mac uint64) error {
	if len(t.devName) == 0 || mac == 0 {
		return errors.New("无效的网卡名称或MAC")
	}
	_ = t.Down()
	macBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(macBuf, mac)
	cname := C.CString(t.devName)
	defer C.free(unsafe.Pointer(cname))
	if C.setPermMac(cname, (*C.uchar)(unsafe.Pointer(&macBuf[0]))) < 0 {
		return errors.New(fmt.Sprintf("设置网卡<%s>MAC失败:%v", t.devName, mac))
	}
	_ = t.Up()
	return nil
}

func (t *TapLinux) GetMac() (uint64, error) {
	if len(t.devName) == 0 {
		return 0, errors.New("invalid if name")
	}
	cname := C.CString(t.devName)
	defer C.free(unsafe.Pointer(cname))
	ret := C.getPermMac(cname)
	if ret < 0 {
		return 0, errors.New(fmt.Sprintf("获取接口<%s>MAC失败,ret:%d", t.devName, int(ret)))
	}
	macBuf := make([]uint8, 8)
	for i := range C.mac {
		macBuf[i] = uint8(C.mac[i])
	}
	return binary.LittleEndian.Uint64(macBuf), nil
}

func (t *TapLinux) SetIpAddr(ip, mask string) error {
	args := []string{
		t.devName, ip, "netmask", mask,
	}
	_, err := exec.Command("ifconfig", args...).CombinedOutput()
	return err
}

func (t *TapLinux) GetIpAddr() (string, string, error) {
	command := "ip addr show dev " + t.devName + " | grep 'inet ' | awk '{print $2}'"
	cmd := exec.Command("sh", "-c", command)
	cidr_byte, err := cmd.Output()
	cidr := strings.Trim(string(cidr_byte), "\n")
	if err != nil {
		return "", "", err
	}
	ip, ipnet, err1 := net.ParseCIDR(cidr)
	if err1 != nil {
		return "", "", err
	}
	return ip.String(), (net.IP)(ipnet.Mask).String(), err1
}

func (t *TapLinux) SetMtu(mtu uint) error {
	args := []string{
		"link", "set", t.devName, "mtu", fmt.Sprintf("%d", mtu),
	}
	_, err := exec.Command("ip", args...).CombinedOutput()
	return err
}

func (t *TapLinux) Up() error {
	args := []string{
		"link", "set", "dev", t.devName, "up",
	}
	_, err := exec.Command("ip", args...).CombinedOutput()
	return err
}

func (t *TapLinux) Down() error {
	args := []string{
		"link", "set", "dev", t.devName, "down",
	}
	_, err := exec.Command("ip", args...).CombinedOutput()
	return err
}

func (t *TapLinux) Close() error {
	return syscall.Close(t.fd)
}

func (t *TapLinux) Read(buf []byte) (int, error) {
	return t.ReadWriteCloser.Read(buf)
}

func (t *TapLinux) Write(buf []byte) (int, error) {
	return t.ReadWriteCloser.Write(buf)
}

// route add -net 172.28.96.0/19 gw 192.168.1.151
func AddRoute(dst, mask, gw string) ([]byte, error) {
	args := []string{
		"add", "-net", dst, "netmask", mask, "gw", gw,
	}
	return exec.Command("route", args...).CombinedOutput()
}

// route del -net 172.28.96.0/19 gw 192.168.1.151
func DelRoute(dst, mask, gw string) ([]byte, error) {
	args := []string{
		"del", "-net", dst, "netmask", mask, "gw", gw,
	}
	return exec.Command("route", args...).CombinedOutput()
}
