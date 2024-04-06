//go:build windows

package serial

import (
	"errors"
	"fmt"
	reg "golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var exePath = ""
var exeFile = "setupc.exe"
var initOk = false

func init() {
	key, err := reg.OpenKey(reg.LOCAL_MACHINE, "SYSTEM\\CurrentControlSet\\Control\\COM Name Arbiter", reg.WRITE)
	if err != nil {
		return
	}
	_ = key.SetBinaryValue("ComDB",
		[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	dir, e := filepath.Abs(filepath.Dir(os.Args[0]))
	if e != nil {
		return
	}
	var a = 0
	if unsafe.Sizeof(a) == 4 { // 32位平台
		exePath = dir + "\\serialport\\x86\\" + exeFile
	} else {
		exePath = dir + "\\serialport\\x64\\" + exeFile
	}
	if _, err0 := os.Stat(exePath); err0 == nil {
		initOk = true
	}
}

type CommPair struct {
	Index uint   `json:"index"`
	ComA  string `json:"comA"`
	ComB  string `json:"comB"`
}

func LoadComPairs() ([]*CommPair, error) {
	if !initOk {
		return nil, errors.New("虚拟网卡驱动异常")
	}
	content, err := exeCmd("list")
	if err != nil {
		return nil, err
	}
	var temp []string
	if strings.Contains(content, "\r\n") {
		temp = strings.Split(content, "\r\n")
	} else {
		temp = strings.Split(content, "\n")
	}
	if len(temp) == 0 {
		return nil, err
	}
	comPairs := make([]*CommPair, 0)
	for i := 0; i < len(temp)/2; i++ {
		pair := &CommPair{}
		str := strings.Trim(temp[2*i], " ")
		if strings.HasPrefix(str, "CNC") {
			comName := strings.Split(str, "=")
			if len(comName) != 2 {
				continue
			}
			regx := regexp.MustCompile("(\\d+)")
			iStr := regx.FindString(comName[0])
			if v, e := strconv.ParseUint(iStr, 10, 16); e != nil {
				continue
			} else {
				pair.Index = uint(v)
				pair.ComA = strings.Trim(comName[1], " ")
			}
		} else {
			continue
		}
		str = strings.Trim(temp[2*i+1], " ")
		if strings.HasPrefix(str, "CNC") {
			comName := strings.Split(str, "=")
			if len(comName) != 2 {
				continue
			}
			pair.ComB = strings.Trim(comName[1], " ")
		} else {
			continue
		}
		comPairs = append(comPairs, pair)
	}
	return comPairs, nil
}

func AddComPair(comA, comB string) (*CommPair, error) {
	if !initOk {
		return nil, errors.New("虚拟网卡驱动异常")
	}
	cmdText := fmt.Sprintf("install PortName=%s PortName=%s", comA, comB)
	res, err := exeCmd(cmdText)
	if err == nil && len(res) > 0 {
		res = strings.Trim(res, " ")
		if strings.HasPrefix(res, "CNC") {
			temp := strings.Split(res, " ")
			regx := regexp.MustCompile("(\\d+)")
			iStr := regx.FindString(temp[0])
			if v, e := strconv.ParseUint(iStr, 10, 16); e != nil {
				return nil, e
			} else {
				return &CommPair{ComA: comA, ComB: comB, Index: uint(v)}, nil
			}
		}
	}
	return nil, errors.New("failed to add com pair")
}

func DelComPair(index uint) error {
	if !initOk {
		return errors.New("虚拟网卡驱动异常")
	}
	cmdText := fmt.Sprintf("remove %d", index)
	_, err := exeCmd(cmdText)
	return err
}

func exeCmd(cmdText string) (string, error) {
	cmd := exec.Command(exePath, cmdText)
	cmd.Dir = strings.Trim(exePath, exeFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	res, err := cmd.Output()
	if err == nil {
		return string(res), err
	} else {
		return "", err
	}
}
