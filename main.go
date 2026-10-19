//go:debug tlsrsakex=1
//go:debug tls3des=1
package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// WebKitGTK con NVIDIA en Wayland puede mostrar la ventana en blanco.
	if runtime.GOOS == "linux" && os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Pluton SQL",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		// En Linux (sobre todo en compositores de mosaico como niri) la ventana puede no traer
		// barra de título ni botón de cerrar, así que la app dibuja la suya (TitleBar.svelte).
		Frameless: runtime.GOOS == "linux",
		// En Linux, con 0 Wails fija el máximo al tamaño del monitor donde arrancó
		// y la ventana no crece al moverla a otro más grande.
		MaxWidth:         16384,
		MaxHeight:        16384,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app.Connections,
			app.Workspaces,
			app.Schema,
			app.Query,
			app.Data,
			app.Structure,
			app.SchemaEdit,
			app.History,
			app.Saved,
			app.Export,
			app.Import,
			app.Dialogs,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
