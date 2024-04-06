package main

import (
	"context"
	"github.com/rodolfoag/gow32"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"runtime"
	"strconv"
	"strings"
	"vilan/app"
	"vilan/common"
	"vilan/config"
	"vilan/model"
	"vilan/protocol"
	"vilan/serial"
	"vilan/service"
)

// WailsApp struct
type WailsApp struct {
	ctx       context.Context
	ti        *TrayIcon
	logEvents []map[string]interface{}
	isInit    bool
}

// NewWailsApp creates a new WailsApp application struct
func NewWailsApp() *WailsApp {
	return &WailsApp{logEvents: make([]map[string]interface{}, 0), isInit: false}
}

// startup is called at application startup
func (a *WailsApp) startup(ctx context.Context) {
	_, err := gow32.CreateMutex("vi_lan_peer")
	if err != nil {
		_, _ = wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
			Type:          wailsRuntime.WarningDialog,
			Title:         "提示",
			Message:       "不能同时启动多个程序!",
			Buttons:       []string{"确定"},
			DefaultButton: "确定",
		})
		os.Exit(-1) // 退出系统
	}
	a.ctx = ctx
	a.ti = NewTrayIcon()
	a.ti.BalloonClickFunc = a.showWindow
	a.ti.TrayClickFunc = a.showWindow
	go a.ti.RunTray()

}

func (a *WailsApp) showWindow() {
	wailsRuntime.WindowShow(a.ctx)
}

// domReady is called after the front-end dom has been loaded
func (a *WailsApp) domReady(context.Context) {
	// 初始化开始...
	if a.isInit {
		if app.RuntimeService != nil {
			wailsRuntime.EventsEmit(a.ctx, "peerState", app.RuntimeService.PeerState())
		}
		return
	}
	a.isInit = true
	// APP配置
	app.Logger = common.NewLogger(true, false, common.Error)
	if err := config.InitAppConfig(); err != nil {
		app.Logger.Error("配置加载失败,程序将以默认配置运行.")
	} else {
		app.Logger = common.NewLogger(config.AppConfig.EnableLog, config.AppConfig.SaveLog, config.AppConfig.LogLevel)
		app.Logger.Info("程序配置加载成功")
	}
	app.Logger.SetFunc(a.onLog)

	// 创建实例
	app.TunTapService = service.NewTunTapService()
	app.RuntimeService = service.NewRuntimeService()
	app.P2pService = service.NewP2pService()
	app.SerialService = serial.NewPortService()
	// TunTapService 初始化
	if config.AppConfig.TapConfig.IpMode == model.Static && config.AppConfig.TapConfig.HwMac != 0 {
		if err := app.TunTapService.Start(); err != nil {
			app.Logger.Error("虚拟网卡初始化失败:", err.Error())
		} else {
			app.Logger.Info("虚拟网卡初始化成功,IP ", common.Uint32toIpV4(config.AppConfig.TapConfig.IpAddr),
				" MAC ", common.Uint64ToMacStr(config.AppConfig.TapConfig.HwMac))
		}
	}

	// Runtime Service 初始化
	if err := app.RuntimeService.Init(); err != nil {
		app.Logger.Error("运行时服务配置初始化失败:", err.Error())
		return
	}
	app.Logger.Info("运行时服务配置初始化成功")

	if err := app.RuntimeService.Start(); err != nil {
		app.Logger.Error("运行时服务启动失败:", err.Error())
		return
	}
	app.Logger.Info("运行时服务启动成功")

	// P2p Service 初始化
	if err := app.P2pService.Start(); err != nil {
		app.Logger.Error("P2P管理服务启动失败:", err.Error())
		return
	}
	app.Logger.Info("P2P管理服务启动成功")

	if config.AppConfig.AllowVisitPort {
		if err := app.SerialService.Start(); err != nil {
			app.Logger.Error("串口服务启动失败:", err.Error())
		} else {
			app.Logger.Info("串口服务启动成功,监听端口:48532")
		}
	} else {
		app.Logger.Info("当前设备串口已禁止远程访问")
	}

	wailsRuntime.EventsEmit(a.ctx, "peerState", app.RuntimeService.PeerState())

}
func (a *WailsApp) beforeClose(context.Context) (prevent bool) {
	dialog, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
		Type:    wailsRuntime.QuestionDialog,
		Title:   "提示",
		Message: "您确定要退出吗?",
	})

	if err != nil {
		return false
	}
	if dialog == "Yes" {
		if app.SerialService != nil {
			app.SerialService.Stop()
		}
		if app.RuntimeService != nil {
			app.RuntimeService.Stop()
		}
		if app.P2pService != nil {
			app.P2pService.Stop()
		}
		if app.Logger != nil {
			app.Logger.Info("服务已关闭")
		}
		return false
	}

	return true
}

// shutdown is called at application termination
func (a *WailsApp) shutdown(context.Context) {

}

func (a *WailsApp) PullPlatform() string {
	return runtime.GOOS
}
func (a *WailsApp) HideWindow() {
	wailsRuntime.WindowHide(a.ctx)
}
func (a *WailsApp) GetSystemVersion() uint32 {
	return common.GetSystemVersion()
}
func (a *WailsApp) GetPeerState() uint {
	if app.RuntimeService == nil {
		return uint(model.StateUnInit)
	}
	return uint(app.RuntimeService.PeerState())
}
func (a *WailsApp) RequestGroupPeers() []*model.PeerInfo {
	if app.RuntimeService == nil {
		return nil
	}
	infos := app.RuntimeService.GetGroupPeers(config.AppConfig.PeerConfig.GroupName, true)
	if infos != nil {
		peers := make([]*model.PeerInfo, len(infos))
		for i := range infos {
			peers[i] = model.ProtoToModel(infos[i])
			if app.P2pService.IsP2P(infos[i].PeerMac) {
				peers[i].ConnectType = 1
			}
		}
		return sortPeers(peers)
	}
	return nil
}

// 排序 先在线 按IP 小到大
func sortPeers(peers []*model.PeerInfo) []*model.PeerInfo {
	if peers == nil || len(peers) == 0 {
		return peers
	}
	onlinePeers := make([]*model.PeerInfo, 0)
	offlinePeers := make([]*model.PeerInfo, 0)
	for i := range peers {
		if peers[i].Online {
			onlinePeers = append(onlinePeers, peers[i])
		} else {
			offlinePeers = append(offlinePeers, peers[i])
		}
	}

	num := len(onlinePeers)
	if num > 0 {
		for i := 0; i < num-1; i++ {
			for j := 0; j < num-i-1; j++ {
				if compareIP(onlinePeers[j].NetAddr, onlinePeers[j+1].NetAddr) {
					temp := onlinePeers[j]
					onlinePeers[j] = onlinePeers[j+1]
					onlinePeers[j+1] = temp
				}
			}
		}
	}
	num = len(offlinePeers)
	if num > 0 {
		for i := 0; i < num-1; i++ {
			for j := 0; j < num-i-1; j++ {
				if compareIP(offlinePeers[j].NetAddr, offlinePeers[j+1].NetAddr) {
					temp := offlinePeers[j]
					offlinePeers[j] = offlinePeers[j+1]
					offlinePeers[j+1] = temp
				}
			}
		}
	}
	return append(onlinePeers, offlinePeers...)
}

func compareIP(ip0, ip1 string) bool {
	ipStr0 := strings.ReplaceAll(ip0, "/", "")
	ipStr0 = strings.ReplaceAll(ipStr0, ".", "")
	ipVal0, err := strconv.ParseUint(ipStr0, 10, 64)
	if err != nil {
		return false
	}
	ipStr1 := strings.ReplaceAll(ip1, "/", "")
	ipStr1 = strings.ReplaceAll(ipStr1, ".", "")
	ipVal1, err := strconv.ParseUint(ipStr1, 10, 64)
	if err != nil {
		return false
	}
	return ipVal0 > ipVal1
}

func (a *WailsApp) GetLinkInfos(macStr string) []*protocol.LinkInfo {
	mac, err := strconv.ParseUint(macStr, 10, 64)
	if err != nil {
		app.Logger.Debug("地址转换错误:", macStr, err)
		return nil
	}
	return app.RuntimeService.GetLinkInfos(mac)
}

func (a *WailsApp) SelectPeer(macStr string) bool {
	mac, err := strconv.ParseUint(macStr, 10, 64)
	if err != nil {
		return false
	}
	return app.RuntimeService.AddRoute(mac)
}

func (a *WailsApp) GetConfig() *model.Config {
	defer func() {
		if err := recover(); err != nil {
			app.Logger.Error("配置请求出错:", err)
		}
	}()
	if app.RuntimeService == nil {
		return nil
	}
	mask := uint32(0xFFFFFFFF) << (32 - config.AppConfig.TapConfig.IpMask)
	conf := &model.Config{
		ServerIp:   config.AppConfig.ServerIp,
		ServerPort: config.AppConfig.ServerPort,
		PeerName:   config.AppConfig.PeerConfig.Name,
		CryptType:  config.AppConfig.PeerConfig.CryptType,
		PeerPwd:    config.AppConfig.PeerConfig.PeerPwd,
		GroupName:  config.AppConfig.PeerConfig.GroupName,
		GroupPwd:   config.AppConfig.PeerConfig.GroupPwd,
		HwMac:      common.Uint64ToMacStr(config.AppConfig.TapConfig.HwMac),
		IpMode:     uint(config.AppConfig.TapConfig.IpMode),
		IpAddr:     common.Uint32toIpV4(config.AppConfig.TapConfig.IpAddr),
		IpMask:     common.Uint32toIpV4(mask),
		EnableLog:  config.AppConfig.EnableLog,
		LogLevel:   byte(config.AppConfig.LogLevel),
	}
	if app.RuntimeService.PeerState() >= model.StateInitOk {
		conf.TabName = app.TunTapService.TapName()
	}
	return conf
}

func (a *WailsApp) SaveConfig(conf *model.Config) string {
	if conf == nil {
		return "配置信息为空"
	}
	if len(conf.PeerName) == 0 {
		return "终端名称不能为空"
	}
	if len(conf.GroupName) == 0 {
		return "网络组名称不能为空"
	}
	//if len(conf.GroupPwd) == 0 {
	//	return "网络组密码不能为空"
	//}
	if conf.CryptType != model.CryptNone && len(conf.PeerPwd) == 0 {
		return "传输密码不能为空"
	}
	_, e := common.IpV4toUint32(conf.ServerIp)
	if e != nil {
		return "服务地址格式错误"
	}
	ipMode := model.AutoAssign
	if conf.IpMode == 0x02 {
		ipMode = model.Static
	}
	peerIp, e := common.IpV4toUint32(conf.IpAddr)
	if e != nil && ipMode != model.AutoAssign {
		return "分配地址格式错误"
	}
	peerMask, e := common.IpV4toUint32(conf.IpMask)
	if e != nil && ipMode != model.AutoAssign {
		return "子网掩码格式错误"
	}

	maskLen := common.MaskBitLen(peerMask)
	mac, e := common.MacStrToUint64(conf.HwMac)
	if e != nil {
		return "MAC地址格式错误"
	}
	c := config.NewConfig()
	if e := config.CopyAppConfigTo(c); e != nil {
		return "配置载入出错"
	}
	c.ServerIp = conf.ServerIp
	c.ServerPort = conf.ServerPort
	c.EnableLog = conf.EnableLog
	c.LogLevel = common.LogType(conf.LogLevel)
	c.PeerConfig.Name = conf.PeerName
	c.PeerConfig.CryptType = conf.CryptType
	c.PeerConfig.PeerPwd = conf.PeerPwd
	c.PeerConfig.GroupName = conf.GroupName
	c.PeerConfig.GroupPwd = conf.GroupPwd
	c.TapConfig.HwMac = mac
	c.TapConfig.IpMode = ipMode
	c.TapConfig.IpAddr = peerIp
	c.TapConfig.IpMask = maskLen
	if err := config.SaveConfig(c); err != nil {
		return "配置保存失败"
	} else {
		needRestart := false
		if config.AppConfig.TapConfig.HwMac != c.TapConfig.HwMac ||
			config.AppConfig.PeerConfig.CryptType != c.PeerConfig.CryptType ||
			(config.AppConfig.PeerConfig.CryptType != model.CryptNone && config.AppConfig.PeerConfig.PeerPwd != c.PeerConfig.PeerPwd) ||
			config.AppConfig.TapConfig.IpMode != c.TapConfig.IpMode {
			needRestart = true
		} else if (config.AppConfig.TapConfig.IpAddr != c.TapConfig.IpAddr ||
			config.AppConfig.TapConfig.IpMask != c.TapConfig.IpMask) && c.TapConfig.IpMode != model.AutoAssign {
			needRestart = true
		}
		if config.AppConfig.ServerIp != c.ServerIp ||
			config.AppConfig.ServerPort != c.ServerPort ||
			config.AppConfig.PeerConfig.GroupName != c.PeerConfig.GroupName ||
			config.AppConfig.PeerConfig.GroupPwd != c.PeerConfig.GroupPwd {
			needRestart = true
		}
		config.AppConfig = c
		if c.EnableLog {
			app.Logger.Enable()
		} else {
			app.Logger.Disable()
		}
		app.Logger.SetLogLevel(c.LogLevel)
		if needRestart {
			if ex := app.RuntimeService.Restart(); ex != nil {
				app.Logger.Error("本地服务重启失败:", ex)
			} else {
				app.Logger.Info("本地服务重启成功!")
			}
		}
		return "配置保存成功"
	}
}

func (a *WailsApp) UpdateState(state model.PeerState) {
	wailsRuntime.EventsEmit(a.ctx, "peerState", state)
}

func (a *WailsApp) UpdateStats(stats *protocol.Statistics) {
	wailsRuntime.EventsEmit(a.ctx, "statistic", stats)
}

func (a *WailsApp) UpdatePeers() {
	infos := app.RuntimeService.GetGroupPeers(config.AppConfig.PeerConfig.GroupName, false)
	if infos != nil {
		peers := make([]*model.PeerInfo, len(infos))
		count := 0
		for i := range infos {
			//if infos[i].DevType == "windows" {
			//	continue
			//}
			peers[count] = model.ProtoToModel(infos[i])
			if app.P2pService.IsP2P(infos[i].PeerMac) {
				peers[count].ConnectType = 1
			}
			count++
		}
		wailsRuntime.EventsEmit(a.ctx, "peerInfos", sortPeers(peers[:count]))
	}
}

func (a *WailsApp) UpdateConfig(peer *model.PeerInfo) map[string]interface{} {
	return app.RuntimeService.UpdateConfig(peer)
}

// 日志处理
func (a *WailsApp) GetHisLog() []map[string]interface{} {
	return a.logEvents
}
func (a *WailsApp) ClearHisLog() {
	a.logEvents = make([]map[string]interface{}, 0)
}

func (a *WailsApp) onLog(logType common.LogType, time string, content string) {
	evt := map[string]interface{}{"logType": logType, "time": time, "content": content}
	if len(a.logEvents) > 100 {
		a.logEvents = a.logEvents[:90]
	}
	a.logEvents = append([]map[string]interface{}{evt}, a.logEvents...)
	wailsRuntime.EventsEmit(a.ctx, "log", evt)
}
