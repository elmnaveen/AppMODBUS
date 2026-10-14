package main

import (
	"embed"
	"log"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"modbusstudio/internal/legacy"
	"modbusstudio/internal/recorder"
	"modbusstudio/internal/settings"
	"modbusstudio/internal/version"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Before anything opens the settings: carry data over from AppMODBUS.
	migrated := legacy.Migrate(settings.LegacyDir(), settings.DefaultDir(), recorder.LegacyDefaultRoot(), recorder.DefaultRoot())
	app := NewApp()
	app.migrated = migrated
	// macOS needs the standard app and Edit menus for Cmd+Q and
	// copy/paste in text fields; the File menu lives in the toolbar.
	var appMenu *menu.Menu
	if goruntime.GOOS == "darwin" {
		appMenu = menu.NewMenu()
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
	}
	err := wails.Run(&options.App{
		Title:            "Modbus Studio " + version.Version,
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		Menu:             appMenu,
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
