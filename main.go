package main

import (
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"log"
	"runtime"
	"runtime/debug"
	"vilan/app"
	"vilan/common"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	fmt.Println("当前程序版本:", app.Version)
	debug.SetMemoryLimit(1024 * 1024 * 50)
	version := common.GetSystemVersion()

	// Create an instance of the wailsApp structure
	wailsApp := NewWailsApp()
	app.WailsApp = wailsApp

	serialApp := NewSerialApp()
	// Create application with options
	//goland:noinspection GoBoolExpressions
	err := wails.Run(&options.App{
		Title:             "Vilan客户端",
		Width:             900,
		Height:            600,
		MinWidth:          760,
		MinHeight:         570,
		DisableResize:     true,
		Fullscreen:        false,
		Frameless:         runtime.GOOS != "darwin",
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		LogLevel:      logger.WARNING,
		OnStartup:     wailsApp.startup,
		OnDomReady:    wailsApp.domReady,
		OnBeforeClose: wailsApp.beforeClose,
		OnShutdown:    wailsApp.shutdown,
		/*
			MaxWidth:          1920,
			MaxHeight:         1280,
		*/
		Bind: []interface{}{
			wailsApp,
			serialApp,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  version > 7, // win7 不支持
			DisableWindowIcon:    true,
			WebviewBrowserPath:   "",
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
