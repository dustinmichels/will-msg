package appservice

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/config"
	"will-msg/internal/parser"
	"will-msg/internal/scanner"
)

// DialogAdapter defines the native dialog capabilities needed by the service.
type DialogAdapter interface {
	OpenMultipleFilesDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) ([]string, error)
	OpenDirectoryDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error)
	OpenFileDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error)
	SaveFileDialog(ctx context.Context, options wailsruntime.SaveDialogOptions) (string, error)
	MessageDialog(ctx context.Context, options wailsruntime.MessageDialogOptions) (string, error)
}

// defaultDialogAdapter delegates directly to Wails runtime dialog helpers.
type defaultDialogAdapter struct{}

func (d *defaultDialogAdapter) OpenMultipleFilesDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) ([]string, error) {
	return wailsruntime.OpenMultipleFilesDialog(ctx, options)
}

func (d *defaultDialogAdapter) OpenDirectoryDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
	return wailsruntime.OpenDirectoryDialog(ctx, options)
}

func (d *defaultDialogAdapter) OpenFileDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
	return wailsruntime.OpenFileDialog(ctx, options)
}

func (d *defaultDialogAdapter) SaveFileDialog(ctx context.Context, options wailsruntime.SaveDialogOptions) (string, error) {
	return wailsruntime.SaveFileDialog(ctx, options)
}

func (d *defaultDialogAdapter) MessageDialog(ctx context.Context, options wailsruntime.MessageDialogOptions) (string, error) {
	return wailsruntime.MessageDialog(ctx, options)
}

// SourceLoader is a function that loads message metadata from a source.
type SourceLoader func(scanner.MessageSource) (parser.MessageMetadata, error)

// PlatformRevealer reveals a file in the platform's file manager.
type PlatformRevealer func(ctx context.Context, path string) error

// defaultPlatformRevealer isolates platform commands behind per-OS helpers without shell invocation.
func defaultPlatformRevealer(ctx context.Context, path string) error {
	cleanPath := filepath.Clean(path)
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.CommandContext(ctx, "open", "-R", cleanPath)
		return cmd.Run()
	case "windows":
		cmd := exec.CommandContext(ctx, "explorer.exe", fmt.Sprintf("/select,%s", cleanPath))
		return cmd.Run()
	case "linux":
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			absPath = cleanPath
		}
		fileURL := (&url.URL{Scheme: "file", Path: absPath}).String()
		// Try D-Bus org.freedesktop.FileManager1 first
		cmd := exec.CommandContext(ctx, "dbus-send", "--session", "--dest=org.freedesktop.FileManager1",
			"--type=method_call", "/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems", "array:string:"+fileURL, "string:")
		if err := cmd.Run(); err != nil {
			// Fallback to xdg-open on parent directory
			dir := filepath.Dir(absPath)
			fallbackCmd := exec.CommandContext(ctx, "xdg-open", dir)
			return fallbackCmd.Run()
		}
		return nil
	default:
		dir := filepath.Dir(cleanPath)
		cmd := exec.CommandContext(ctx, "xdg-open", dir)
		return cmd.Run()
	}
}

// DownloadsDirProvider returns the user's Downloads directory path.
type DownloadsDirProvider func() (string, error)

// defaultDownloadsDirProvider determines the user Downloads directory location.
func defaultDownloadsDirProvider() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	downloads := filepath.Join(home, "Downloads")
	return downloads, nil
}

// ConfigLoader is a function that loads the active RuleConfig.
type ConfigLoader func() config.RuleConfig

// ConfigSaver is a function that persists a RuleConfig.
type ConfigSaver func(cfg config.RuleConfig) error
