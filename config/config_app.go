package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"vilan/common"
	"vilan/model"
)

var AppConfig *model.AppConfig

func NewConfig() *model.AppConfig {
	return &model.AppConfig{}
}

func InitAppConfig() (err error) {
	var (
		content []byte
		conf    model.AppConfig
	)
	filename := common.GetConfigPath() + "config_app.json"

	if content, err = ioutil.ReadFile(filename); err != nil {
		fmt.Println(err)
		return
	}
	hostname, e := os.Hostname()
	if e != nil || len(hostname) == 0 {
		hostname = "vilan_peer"
	}
	needSave := false
	if err = json.Unmarshal(content, &conf); err == nil {
		AppConfig = &conf
		if AppConfig.PeerConfig == nil {
			AppConfig.PeerConfig = &model.PeerConfig{Name: hostname, GroupName: "default_group", PeerPwd: "group_pwd"}
			needSave = true
		}
		if len(AppConfig.PeerConfig.Name) == 0 {
			AppConfig.PeerConfig.Name = hostname
			needSave = true
		}
		if AppConfig.TapConfig == nil {
			AppConfig.TapConfig = &model.TapConfig{Name: "vilan", HwMac: 0, HwMacStr: "", IpMode: model.AutoAssign, IpAddr: 0, IpMask: 0, DevType: model.TAP}
			needSave = true
		} else {
			if hw, e0 := strconv.ParseUint(AppConfig.TapConfig.HwMacStr, 10, 64); e0 == nil {
				AppConfig.TapConfig.HwMac = hw
			}
			if ip, e0 := IpV4toUint32(AppConfig.TapConfig.IpAddrStr); e0 == nil {
				AppConfig.TapConfig.IpAddr = ip
			}
		}
		peerPwd, e := base64.StdEncoding.DecodeString(AppConfig.PeerConfig.PeerPwd)
		if e == nil {
			pwd := string(peerPwd)
			AppConfig.PeerConfig.PeerPwd = pwd
		}
		groupPwd, e := base64.StdEncoding.DecodeString(AppConfig.PeerConfig.GroupPwd)
		if e == nil {
			pwd := string(groupPwd)
			AppConfig.PeerConfig.GroupPwd = pwd
		}
		if needSave {
			_ = SaveAppConfig()
		}
	} else { // 默认值
		AppConfig = &conf
		AppConfig.ServerIp = "192.168.1.100" // 服务器IP
		AppConfig.ServerPort = 3000          // 服务器监听端口
		AppConfig.Heartbeat = 30
		AppConfig.Offline = 90
		AppConfig.MaxPacketSize = 65535
		AppConfig.PacketNum = 128
		AppConfig.P2pTryCount = 3
		AppConfig.P2pRetryInterval = 3
		AppConfig.AllowVisitPort = false
		AppConfig.EnableLog = true
		AppConfig.SaveLog = true
		AppConfig.LogLevel = common.Info
		pwd := base64.StdEncoding.EncodeToString([]byte("default_pwd"))
		gPwd := base64.StdEncoding.EncodeToString([]byte("12345678"))
		AppConfig.PeerConfig = &model.PeerConfig{Name: hostname, GroupName: "default_group", GroupPwd: gPwd, PeerPwd: pwd, CryptType: model.CryptAES}
		AppConfig.TapConfig = &model.TapConfig{Name: "vilan", HwMac: 0, HwMacStr: "", IpMode: model.AutoAssign, IpAddr: 0, IpMask: 0, DevType: model.TAP}
		_ = SaveAppConfig()
		AppConfig.PeerConfig.PeerPwd = "default_pwd"
		AppConfig.PeerConfig.GroupPwd = "12345678"
		return err
	}
	return
}

func SaveAppConfig() (err error) {
	peerPwd := AppConfig.PeerConfig.PeerPwd
	groupPwd := AppConfig.PeerConfig.GroupPwd
	AppConfig.PeerConfig.PeerPwd = base64.StdEncoding.EncodeToString([]byte(AppConfig.PeerConfig.PeerPwd))
	AppConfig.PeerConfig.GroupPwd = base64.StdEncoding.EncodeToString([]byte(AppConfig.PeerConfig.GroupPwd))

	AppConfig.TapConfig.HwMacStr = fmt.Sprintf("%d", AppConfig.TapConfig.HwMac)
	AppConfig.TapConfig.IpAddrStr = Uint32toIpV4(AppConfig.TapConfig.IpAddr)

	content, err := json.Marshal(*AppConfig)
	AppConfig.PeerConfig.PeerPwd = peerPwd
	AppConfig.PeerConfig.GroupPwd = groupPwd
	if err != nil {
		return err
	}
	filename := common.GetConfigPath() + "config_app.json"
	return ioutil.WriteFile(filename, content, 0664)
}

func SaveConfig(conf *model.AppConfig) (err error) {
	peerPwd := conf.PeerConfig.PeerPwd
	groupPwd := conf.PeerConfig.GroupPwd
	conf.PeerConfig.PeerPwd = base64.StdEncoding.EncodeToString([]byte(conf.PeerConfig.PeerPwd))
	conf.PeerConfig.GroupPwd = base64.StdEncoding.EncodeToString([]byte(conf.PeerConfig.GroupPwd))
	conf.TapConfig.HwMacStr = fmt.Sprintf("%d", conf.TapConfig.HwMac)
	conf.TapConfig.IpAddrStr = Uint32toIpV4(conf.TapConfig.IpAddr)

	content, err := json.Marshal(*conf)
	conf.PeerConfig.PeerPwd = peerPwd
	conf.PeerConfig.GroupPwd = groupPwd
	if err != nil {
		return err
	}
	filename := common.GetConfigPath() + "config_app.json"
	return ioutil.WriteFile(filename, content, 0664)
}
func CopyAppConfigTo(conf *model.AppConfig) error {
	if conf == nil {
		return errors.New("目标配置对象为空")
	}
	conf.ServerIp = AppConfig.ServerIp
	conf.ServerPort = AppConfig.ServerPort
	conf.Heartbeat = AppConfig.Heartbeat
	conf.Offline = AppConfig.Offline
	conf.P2pTryCount = AppConfig.P2pTryCount
	conf.P2pRetryInterval = AppConfig.P2pRetryInterval
	conf.MaxPacketSize = AppConfig.MaxPacketSize
	conf.PacketNum = AppConfig.PacketNum
	conf.SaveLog = AppConfig.SaveLog
	conf.AllowVisitPort = AppConfig.AllowVisitPort
	conf.EnableLog = AppConfig.EnableLog
	conf.LogLevel = AppConfig.LogLevel
	tap := AppConfig.TapConfig
	conf.TapConfig = &model.TapConfig{Name: tap.Name, HwMac: tap.HwMac, HwMacStr: tap.HwMacStr, IpMode: tap.IpMode, IpAddr: tap.IpAddr, IpMask: tap.IpMask, DevType: tap.DevType}
	pc := AppConfig.PeerConfig
	conf.PeerConfig = &model.PeerConfig{Name: pc.Name, GroupName: pc.GroupName, GroupPwd: pc.GroupPwd, PeerPwd: pc.PeerPwd, CryptType: pc.CryptType}
	return nil
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
