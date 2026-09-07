package gui

import (
	"context"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

//go:embed truck.png
var truckPNGBytes []byte

var truckResource = fyne.NewStaticResource("truck.png", truckPNGBytes)

var defaultEngine atomic.Pointer[engine.RuleEngine]

func init() {
	defaultEngine.Store(engine.NewRuleEngine(config.DefaultRuleConfig()))
}

// DefaultEngine returns the active snapshot of the default rule engine.
func DefaultEngine() *engine.RuleEngine {
	eng := defaultEngine.Load()
	if eng == nil {
		eng = engine.NewRuleEngine(config.DefaultRuleConfig())
	}
	return eng
}

// SetDefaultEngine updates the active default rule engine atomically.
func SetDefaultEngine(eng *engine.RuleEngine) {
	if eng == nil {
		eng = engine.NewRuleEngine(config.DefaultRuleConfig())
	}
	defaultEngine.Store(eng)
}

// ReloadDefaultEngine reloads configuration from disk and updates the default rule engine atomically.
func ReloadDefaultEngine() {
	SetDefaultEngine(engine.NewRuleEngine(config.LoadConfig()))
}

func parseMsgSources(sources []scanner.MessageSource) ([]engine.Record, error) {
	eng := DefaultEngine()
	allRecords := make([]engine.Record, 0)

	for _, src := range sources {
		meta, err := scanner.LoadSource(src)
		if err != nil {
			log.Printf("warning: skipping %s: %v", src.DisplayName, err)
			continue
		}

		records := eng.ParseRecords(meta)
		allRecords = append(allRecords, records...)
	}

	return allRecords, nil
}

type customTheme struct {
	fyne.Theme
}

func (c customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 108, G: 185, B: 68, A: 255} // Truck green
	}
	if name == theme.ColorNameFocus {
		return color.NRGBA{R: 85, G: 155, B: 50, A: 255} // Dark truck green
	}
	if name == theme.ColorNameSelection {
		return color.NRGBA{R: 108, G: 185, B: 68, A: 80} // Soft green overlay
	}
	if name == theme.ColorNameBackground {
		if variant == theme.VariantDark {
			return color.NRGBA{R: 15, G: 23, B: 42, A: 255} // Slate-900
		}
		return color.NRGBA{R: 248, G: 250, B: 252, A: 255} // Slate-50
	}
	if name == theme.ColorNameButton {
		if variant == theme.VariantDark {
			return color.NRGBA{R: 30, G: 41, B: 59, A: 255} // Slate-800
		}
		return color.NRGBA{R: 220, G: 237, B: 210, A: 255} // Soft green-tinted button
	}
	if name == theme.ColorNameInputBackground {
		if variant == theme.VariantDark {
			return color.NRGBA{R: 30, G: 41, B: 59, A: 255} // Slate-800
		}
		return color.NRGBA{R: 241, G: 248, B: 237, A: 255} // Soft green-tinted input
	}
	return c.Theme.Color(name, variant)
}

func getDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	downloadsDir := filepath.Join(home, "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		return home
	}
	return downloadsDir
}

func revealFile(filePath string, a fyne.App) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		cmd = exec.Command("explorer.exe", "/select,"+filePath)
	case "linux":
		fileURI := "file://" + filePath
		cmd = exec.Command("dbus-send", "--session", "--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", "array:string:"+fileURI, "string:\"\"")
		if err := cmd.Run(); err == nil {
			return
		}
		cmd = exec.Command("xdg-open", filepath.Dir(filePath))
	default:
		if dirURI := storage.NewFileURI(filepath.Dir(filePath)); dirURI != nil {
			if u, err := url.Parse(dirURI.String()); err == nil {
				_ = a.OpenURL(u)
			}
		}
		return
	}

	if cmd != nil {
		err := cmd.Run()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok || runtime.GOOS != "windows" {
				if dirURI := storage.NewFileURI(filepath.Dir(filePath)); dirURI != nil {
					if u, err := url.Parse(dirURI.String()); err == nil {
						_ = a.OpenURL(u)
					}
				}
			}
		}
	}
}

// RunGUI launches the interactive Fyne desktop user interface.
func RunGUI() {
	ReloadDefaultEngine()
	a := app.New()
	a.Settings().SetTheme(customTheme{Theme: theme.DefaultTheme()})

	w := a.NewWindow("Outlook MSG to CSV Parser")
	w.Resize(fyne.NewSize(950, 700))

	ctx, cancelAnimation := context.WithCancel(context.Background())
	w.SetOnClosed(func() {
		cancelAnimation()
	})

	var currentSources []scanner.MessageSource
	var displayNames []string
	var csvData [][]string
	var updateSource func(path string)

	// Status Bar
	statusBarBg := canvas.NewRectangle(color.NRGBA{R: 30, G: 41, B: 59, A: 255}) // Slate-800
	statusBarBg.SetMinSize(fyne.NewSize(0, 30))

	// Pacman bottom chomping animation
	statusBarAnimationContainer := container.NewWithoutLayout()

	var mouthAngle float64 = 0.0
	pacman := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		cx := float64(w) / 2.0
		cy := float64(h) / 2.0
		radius := float64(w) / 2.0
		if float64(h)/2.0 < radius {
			radius = float64(h) / 2.0
		}

		dx := float64(x) - cx
		dy := float64(y) - cy
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > radius {
			return color.Transparent
		}

		angle := math.Atan2(dy, dx)
		if math.Abs(angle) > math.Pi-mouthAngle {
			return color.Transparent
		}

		return color.NRGBA{R: 255, G: 255, B: 0, A: 255} // Pure Yellow
	})
	pacman.Resize(fyne.NewSize(24, 24))
	statusBarAnimationContainer.Add(pacman)

	// Dots
	var dots []*canvas.Circle
	for i := 0; i < 40; i++ {
		dot := canvas.NewCircle(color.NRGBA{R: 255, G: 255, B: 0, A: 255})
		dot.Resize(fyne.NewSize(6, 6))
		dots = append(dots, dot)
		statusBarAnimationContainer.Add(dot)
	}

	var pacmanX float32 = -9999

	go func() {
		ticker := time.NewTicker(time.Millisecond * 30)
		defer ticker.Stop()

		var t float64 = 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			t += 0.13 // mouth chomping speed
			width := statusBarBg.Size().Width
			if width <= 0 {
				continue
			}

			if pacmanX == -9999 {
				pacmanX = width + 30
			}

			pacmanX -= 1.4 // movement speed
			if pacmanX < -30 {
				pacmanX = width + 30
			}

			mouthAngle = math.Abs(math.Sin(t)) * 0.8

			barHeight := statusBarBg.Size().Height
			if barHeight <= 0 {
				barHeight = 30
			}
			pacmanY := (barHeight - 24) / 2
			dotY := (barHeight - 6) / 2

			fyne.Do(func() {
				pacman.Move(fyne.NewPos(pacmanX, pacmanY))
				pacman.Refresh()

				dotSpacing := float32(50.0)
				for i, dot := range dots {
					dotX := float32(i+1) * dotSpacing
					dot.Move(fyne.NewPos(dotX+9, dotY))

					if pacmanX < dotX {
						dot.Hide()
					} else {
						if dotX < width {
							dot.Show()
						} else {
							dot.Hide()
						}
					}
					dot.Refresh()
				}
				statusBarAnimationContainer.Refresh()
			})
		}
	}()

	statusBarContainer := container.NewStack(
		statusBarBg,
		statusBarAnimationContainer,
	)

	selectedPathLabel := widget.NewLabel("Selected: None")
	selectedPathLabel.Wrapping = fyne.TextWrapWord

	fileCountLabel := widget.NewLabel("Found 0 .msg files")
	fileCountLabel.TextStyle = fyne.TextStyle{Italic: true}

	list := widget.NewList(
		func() int { return len(displayNames) },
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.DocumentIcon())
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, icon, nil, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			box := item.(*fyne.Container)
			var label *widget.Label
			for _, obj := range box.Objects {
				if l, ok := obj.(*widget.Label); ok {
					label = l
					break
				}
			}
			if label != nil {
				label.SetText(displayNames[id])
			}
		},
	)

	previewTable := widget.NewTable(
		func() (int, int) {
			if len(csvData) == 0 {
				return 0, 0
			}
			return len(csvData), len(csvData[0])
		},
		func() fyne.CanvasObject {
			bg := canvas.NewRectangle(color.Transparent)
			l := widget.NewLabel("template text")
			l.Truncation = fyne.TextTruncateEllipsis
			return container.NewStack(bg, l)
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			stack := cell.(*fyne.Container)
			bg := stack.Objects[0].(*canvas.Rectangle)
			label := stack.Objects[1].(*widget.Label)
			label.SetText(csvData[id.Row][id.Col])

			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				if a.Settings().ThemeVariant() == theme.VariantDark {
					bg.FillColor = color.NRGBA{R: 30, G: 41, B: 59, A: 255}
				} else {
					bg.FillColor = color.NRGBA{R: 220, G: 245, B: 195, A: 255} // Light green table header
				}
			} else {
				label.TextStyle = fyne.TextStyle{}
				bg.FillColor = color.Transparent
			}
			label.Refresh()
			bg.Refresh()
		},
	)

	colWidths := []float32{150, 150, 120, 120, 80, 50, 250, 150, 120, 80, 80}
	for i, colW := range colWidths {
		previewTable.SetColumnWidth(i, colW)
	}

	runButton := widget.NewButtonWithIcon("Run Parser", theme.ConfirmIcon(), nil)
	runButton.Importance = widget.HighImportance
	runButton.Disable()

	downloadButton := widget.NewButtonWithIcon("Download CSV", theme.DownloadIcon(), nil)
	downloadButton.Importance = widget.HighImportance
	downloadButton.Disable()

	saveAsButton := widget.NewButtonWithIcon("Save As...", theme.DocumentSaveIcon(), nil)
	saveAsButton.Importance = widget.HighImportance
	saveAsButton.Disable()

	bodyContainer := container.NewStack()

	var showWelcome func()
	var showWorkspace func()

	updateSource = func(path string) {
		sources, err := scanner.FindSources(path)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		currentSources = sources
		displayNames = make([]string, len(sources))
		for i, src := range sources {
			displayNames[i] = src.DisplayName
		}

		selectedPathLabel.SetText("Selected: " + path)
		fileCountLabel.SetText(fmt.Sprintf("Found %d .msg files", len(sources)))
		list.Refresh()

		if len(sources) > 0 {
			runButton.Enable()
			showWorkspace()
		} else {
			runButton.Disable()
			dialog.ShowInformation("No Files Found", "No .msg files were found in the selected source.", w)
		}
	}

	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			path := uris[0].Path()
			if path != "" {
				updateSource(path)
			}
		}
	})

	runButton.OnTapped = func() {
		runButton.Disable()
		downloadButton.Disable()
		saveAsButton.Disable()

		sourcesToParse := append([]scanner.MessageSource(nil), currentSources...)
		go func() {
			records, err := parseMsgSources(sourcesToParse)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, w)
					runButton.Enable()
				})
				return
			}

			if len(records) == 0 {
				fyne.Do(func() {
					dialog.ShowInformation("No Data", "No structured records found in selected files.", w)
					runButton.Enable()
				})
				return
			}

			csvRows := [][]string{engine.CSVHeaders}
			for _, rec := range records {
				csvRows = append(csvRows, rec.ToRow())
			}

			fyne.Do(func() {
				csvData = csvRows
				previewTable.Refresh()
				downloadButton.Enable()
				saveAsButton.Enable()
				runButton.Enable()
			})
		}()
	}

	downloadButton.OnTapped = func() {
		downloadsDir := getDownloadsDir()
		datetime := time.Now().Format("2006-01-02_150405")
		filename := fmt.Sprintf("msg_parsed_%s.csv", datetime)
		filePath := filepath.Join(downloadsDir, filename)

		file, err := os.Create(filePath)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		csvWriter := csv.NewWriter(file)
		writeErr := csvWriter.WriteAll(csvData)
		closeErr := file.Close()
		if writeErr != nil {
			dialog.ShowError(writeErr, w)
			return
		}
		if closeErr != nil {
			dialog.ShowError(closeErr, w)
			return
		}

		title := "CSV Saved Automatically"
		msgLabel := widget.NewLabel("Your CSV has been automatically saved.")

		fileInfo := widget.NewForm(
			widget.NewFormItem("File Name:", widget.NewLabel(filename)),
			widget.NewFormItem("Saved To:", widget.NewLabel(downloadsDir)),
		)

		showInFolderBtn := widget.NewButtonWithIcon("Show in Folder", theme.FolderOpenIcon(), func() {
			revealFile(filePath, a)
		})
		showInFolderBtn.Importance = widget.HighImportance

		dialogContent := container.NewVBox(
			msgLabel,
			fileInfo,
			layout.NewSpacer(),
			container.NewHBox(layout.NewSpacer(), showInFolderBtn, layout.NewSpacer()),
		)

		d := dialog.NewCustom(title, "OK", dialogContent, w)
		d.Resize(fyne.NewSize(500, 200))
		d.Show()
	}

	saveAsButton.OnTapped = func() {
		datetime := time.Now().Format("2006-01-02_150405")
		defaultName := fmt.Sprintf("msg_parsed_%s.csv", datetime)
		go func() {
			path, err := zenity.SelectFileSave(
				zenity.Title("Save CSV As..."),
				zenity.Filename(defaultName),
				zenity.FileFilters{
					{Name: "CSV files", Patterns: []string{"*.csv"}},
				},
			)
			if err != nil {
				if !errors.Is(err, zenity.ErrCanceled) {
					fyne.Do(func() {
						dialog.ShowError(err, w)
					})
				}
				return
			}

			file, err := os.Create(path)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, w)
				})
				return
			}

			csvWriter := csv.NewWriter(file)
			writeErr := csvWriter.WriteAll(csvData)
			closeErr := file.Close()
			if writeErr != nil {
				fyne.Do(func() {
					dialog.ShowError(writeErr, w)
				})
				return
			}
			if closeErr != nil {
				fyne.Do(func() {
					dialog.ShowError(closeErr, w)
				})
				return
			}

			filename := filepath.Base(path)
			dir := filepath.Dir(path)

			fyne.Do(func() {
				title := "CSV Saved"
				msgLabel := widget.NewLabel("Your CSV has been saved.")

				fileInfo := widget.NewForm(
					widget.NewFormItem("File Name:", widget.NewLabel(filename)),
					widget.NewFormItem("Saved To:", widget.NewLabel(dir)),
				)

				showInFolderBtn := widget.NewButtonWithIcon("Show in Folder", theme.FolderOpenIcon(), func() {
					revealFile(path, a)
				})
				showInFolderBtn.Importance = widget.HighImportance

				dialogContent := container.NewVBox(
					msgLabel,
					fileInfo,
					layout.NewSpacer(),
					container.NewHBox(layout.NewSpacer(), showInFolderBtn, layout.NewSpacer()),
				)

				d := dialog.NewCustom(title, "OK", dialogContent, w)
				d.Resize(fyne.NewSize(500, 200))
				d.Show()
			})
		}()
	}

	headerBg := canvas.NewRectangle(color.NRGBA{R: 108, G: 185, B: 68, A: 255}) // Truck green
	headerBg.SetMinSize(fyne.NewSize(0, 50))

	truckImg := canvas.NewImageFromResource(truckResource)
	truckImg.FillMode = canvas.ImageFillContain
	truckImg.SetMinSize(fyne.NewSize(40, 40))

	headerTitle := canvas.NewText("  Outlook MSG Parser", color.White)
	headerTitle.TextSize = 18
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}

	headerSubtitle := canvas.NewText("Feed me your msg files, Will", color.NRGBA{R: 220, G: 245, B: 195, A: 255})
	headerSubtitle.TextSize = 13
	headerSubtitle.TextStyle = fyne.TextStyle{Italic: true}

	rulesButton := widget.NewButtonWithIcon("Rules & Labels", theme.SettingsIcon(), func() {
		ShowRuleEditorWindow(a, w, nil)
	})

	headerContent := container.NewHBox(
		truckImg,
		headerTitle,
		layout.NewSpacer(),
		rulesButton,
		canvas.NewText("  ", color.Transparent),
		headerSubtitle,
		canvas.NewText("   ", color.Transparent),
	)

	header := container.NewStack(
		headerBg,
		container.NewPadded(headerContent),
	)

	uploadIcon := canvas.NewImageFromResource(theme.UploadIcon())
	uploadIcon.FillMode = canvas.ImageFillContain
	uploadIcon.SetMinSize(fyne.NewSize(80, 80))

	headline := canvas.NewText("Feed me your msg files, Will", color.NRGBA{R: 108, G: 185, B: 68, A: 255}) // Truck green
	headline.TextSize = 28
	headline.TextStyle = fyne.TextStyle{Bold: true}
	headline.Alignment = fyne.TextAlignCenter

	description := widget.NewLabel("Drop or select. Accepts .msg files, folders containing .msg files, or .zip archives.")
	description.Alignment = fyne.TextAlignCenter
	description.TextStyle = fyne.TextStyle{Italic: true}

	welcomeSelectBtn := widget.NewButtonWithIcon("Select file or folder", theme.FolderOpenIcon(), func() {
		go func() {
			path, err := zenity.SelectFile(
				zenity.Title("Select file or folder"),
				zenity.FileFilters{
					{Name: "MSG/ZIP files", Patterns: []string{"*.msg", "*.zip"}},
				},
			)
			if err != nil {
				if !errors.Is(err, zenity.ErrCanceled) {
					fyne.Do(func() {
						dialog.ShowError(err, w)
					})
				}
				return
			}
			fyne.Do(func() {
				updateSource(path)
			})
		}()
	})
	welcomeSelectBtn.Importance = widget.HighImportance

	welcomeButtons := container.NewHBox(
		layout.NewSpacer(),
		welcomeSelectBtn,
		layout.NewSpacer(),
	)

	strokeColor := color.NRGBA{R: 108, G: 185, B: 68, A: 255} // Truck green

	var dropZoneBg *canvas.Rectangle
	if a.Settings().ThemeVariant() == theme.VariantDark {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 30, G: 41, B: 59, A: 255})
	} else {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 232, G: 247, B: 220, A: 255}) // Light green tint
	}
	dropZoneBg.CornerRadius = 16
	dropZoneBg.StrokeColor = strokeColor
	dropZoneBg.StrokeWidth = 2
	dropZoneBg.SetMinSize(fyne.NewSize(650, 400))
	dropZoneContent := container.NewVBox(
		layout.NewSpacer(),
		uploadIcon,
		canvas.NewText(" ", color.Transparent),
		headline,
		description,
		canvas.NewText(" ", color.Transparent),
		welcomeButtons,
		layout.NewSpacer(),
	)

	dropZoneStack := container.NewStack(
		dropZoneBg,
		container.NewPadded(dropZoneContent),
	)

	welcomeScreen := container.NewCenter(dropZoneStack)

	backBtn := widget.NewButtonWithIcon("Load New Source", theme.HomeIcon(), func() {
		currentSources = nil
		displayNames = nil
		csvData = nil
		selectedPathLabel.SetText("Selected: None")
		fileCountLabel.SetText("Found 0 .msg files")
		runButton.Disable()
		downloadButton.Disable()
		saveAsButton.Disable()
		list.Refresh()
		previewTable.Refresh()
		showWelcome()
	})

	listTitle := widget.NewLabelWithStyle("Detected Files:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	leftTopArea := container.NewVBox(
		backBtn,
		canvas.NewText(" ", color.Transparent),
		widget.NewLabelWithStyle("Selected MSG Source:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		selectedPathLabel,
		fileCountLabel,
		canvas.NewText(" ", color.Transparent),
		listTitle,
	)

	leftPanel := container.NewBorder(
		leftTopArea,
		runButton,
		nil,
		nil,
		list,
	)

	rightPanel := container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("CSV Preview:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			downloadButton,
			saveAsButton,
		),
		nil,
		nil,
		nil,
		previewTable,
	)

	workspaceSplit := container.NewHSplit(leftPanel, rightPanel)
	workspaceSplit.Offset = 0.35

	showWelcome = func() {
		bodyContainer.Objects = []fyne.CanvasObject{welcomeScreen}
		bodyContainer.Refresh()
	}

	showWorkspace = func() {
		bodyContainer.Objects = []fyne.CanvasObject{workspaceSplit}
		bodyContainer.Refresh()
	}

	showWelcome()

	mainLayout := container.NewBorder(
		header,
		statusBarContainer,
		nil,
		nil,
		bodyContainer,
	)

	w.SetContent(mainLayout)
	w.ShowAndRun()
}
