package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/appservice"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the backend app service
	svc := appservice.NewService()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Outlook MSG to CSV Parser",
		Width:     950,
		Height:    700,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: options.NewRGB(248, 250, 252),
		OnStartup:        svc.Startup,
		OnBeforeClose: func(ctx context.Context) bool {
			if svc.IsRulesDirty() {
				result, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
					Type:          runtime.QuestionDialog,
					Title:         "Unsaved Rules Changes",
					Message:       "You have unsaved changes to your rules. Are you sure you want to exit?",
					Buttons:       []string{"Exit", "Cancel"},
					DefaultButton: "Cancel",
					CancelButton:  "Cancel",
				})
				if err != nil || result != "Exit" {
					return true // prevent shutdown
				}
			}
			return false // allow shutdown
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []any{
			svc,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
