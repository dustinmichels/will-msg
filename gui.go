package main

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
)

type msgSource struct {
	Path        string // path on disk OR path inside zip
	InZip       bool   // whether it's inside a zip
	ZipPath     string // path to the zip file itself
	DisplayName string // name shown in the UI list
}

func loadMessageFromZip(src msgSource) (messageMetadata, error) {
	r, err := zip.OpenReader(src.ZipPath)
	if err != nil {
		return messageMetadata{}, err
	}
	defer r.Close()

	var zipFile *zip.File
	for _, f := range r.File {
		if f.Name == src.Path {
			zipFile = f
			break
		}
	}
	if zipFile == nil {
		return messageMetadata{}, fmt.Errorf("file not found in zip: %s", src.Path)
	}

	rc, err := zipFile.Open()
	if err != nil {
		return messageMetadata{}, err
	}
	defer rc.Close()

	tempFile, err := os.CreateTemp("", "msg-*.msg")
	if err != nil {
		return messageMetadata{}, err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(tempFile, rc); err != nil {
		tempFile.Close()
		return messageMetadata{}, err
	}
	if err := tempFile.Close(); err != nil {
		return messageMetadata{}, err
	}

	return loadMessage(tempPath)
}

func shouldIgnore(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") || strings.EqualFold(part, "__MACOSX") {
			return true
		}
	}
	return false
}

func findMsgFiles(path string) ([]msgSource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var sources []msgSource

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".msg" {
			sources = append(sources, msgSource{
				Path:        path,
				DisplayName: filepath.Base(path),
			})
		} else if ext == ".zip" {
			r, err := zip.OpenReader(path)
			if err != nil {
				return nil, fmt.Errorf("open zip: %w", err)
			}
			defer r.Close()

			for _, f := range r.File {
				if f.FileInfo().IsDir() {
					continue
				}
				if shouldIgnore(f.Name) {
					continue
				}
				if strings.EqualFold(filepath.Ext(f.Name), ".msg") {
					sources = append(sources, msgSource{
						Path:        f.Name,
						InZip:       true,
						ZipPath:     path,
						DisplayName: f.Name + " (in " + filepath.Base(path) + ")",
					})
				}
			}
		} else {
			return nil, fmt.Errorf("unsupported file extension: %s", ext)
		}
	} else {
		err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(path, p)
			if err != nil {
				rel = filepath.Base(p)
			}
			if shouldIgnore(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".msg") {
				sources = append(sources, msgSource{
					Path:        p,
					DisplayName: rel,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return sources, nil
}

func parseMsgSources(sources []msgSource) ([]record, error) {
	allRecords := make([]record, 0)

	for _, src := range sources {
		var meta messageMetadata
		var err error

		if src.InZip {
			meta, err = loadMessageFromZip(src)
		} else {
			meta, err = loadMessage(src.Path)
		}

		if err != nil {
			log.Printf("warning: skipping %s: %v", src.DisplayName, err)
			continue
		}

		meta.SourceFile = filepath.Base(src.Path)

		records := parseRecords(meta)
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
		// Try dbus show items first
		fileURI := "file://" + filePath
		cmd = exec.Command("dbus-send", "--session", "--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", "array:string:"+fileURI, "string:\"\"")
		if err := cmd.Run(); err == nil {
			return
		}
		// Fallback to xdg-open the parent directory
		cmd = exec.Command("xdg-open", filepath.Dir(filePath))
	default:
		// Fallback to generic open folder
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
			// On Windows, explorer.exe /select exits with a non-zero code even on success.
			// Only fallback if the error is not an ExitError (meaning explorer.exe couldn't launch).
			if _, ok := err.(*exec.ExitError); !ok || runtime.GOOS != "windows" {
				// Fallback to generic open folder
				if dirURI := storage.NewFileURI(filepath.Dir(filePath)); dirURI != nil {
					if u, err := url.Parse(dirURI.String()); err == nil {
						_ = a.OpenURL(u)
					}
				}
			}
		}
	}
}

func runGUI() {
	a := app.New()
	a.Settings().SetTheme(customTheme{Theme: theme.DefaultTheme()})

	w := a.NewWindow("Outlook MSG to CSV Parser")
	w.Resize(fyne.NewSize(950, 700))

	var currentSources []msgSource
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
		// Pacman is going right to left, so the mouth faces left (angle around Pi or -Pi)
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
		for range ticker.C {
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

	colWidths := []float32{150, 150, 120, 120, 80, 50, 250, 150, 120, 80}
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
		sources, err := findMsgFiles(path)
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

		go func() {
			records, err := parseMsgSources(currentSources)
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

			csvRows := [][]string{csvHeaders}
			for _, rec := range records {
				csvRows = append(csvRows, rec.toRow())
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
		defer file.Close()

		csvWriter := csv.NewWriter(file)
		err = csvWriter.WriteAll(csvData)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Show custom dialog with file name, directory, and action buttons to open/reveal
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
			defer file.Close()

			csvWriter := csv.NewWriter(file)
			err = csvWriter.WriteAll(csvData)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, w)
				})
				return
			}

			fyne.Do(func() {
			})
		}()
	}

	headerBg := canvas.NewRectangle(color.NRGBA{R: 108, G: 185, B: 68, A: 255}) // Truck green
	headerBg.SetMinSize(fyne.NewSize(0, 50))

	truckImg := canvas.NewImageFromFile("truck.png")
	truckImg.FillMode = canvas.ImageFillContain
	truckImg.SetMinSize(fyne.NewSize(40, 40))

	headerTitle := canvas.NewText("  Outlook MSG Parser", color.White)
	headerTitle.TextSize = 18
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}

	headerSubtitle := canvas.NewText("Feed me your msg files, Will", color.NRGBA{R: 220, G: 245, B: 195, A: 255})
	headerSubtitle.TextSize = 13
	headerSubtitle.TextStyle = fyne.TextStyle{Italic: true}

	headerContent := container.NewHBox(
		truckImg,
		headerTitle,
		layout.NewSpacer(),
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

	var dropZoneBg *canvas.Rectangle
	if a.Settings().ThemeVariant() == theme.VariantDark {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 30, G: 41, B: 59, A: 255})
	} else {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 232, G: 247, B: 220, A: 255}) // Light green tint
	}
	dropZoneBg.CornerRadius = 16
	dropZoneBg.SetMinSize(fyne.NewSize(650, 400))

	strokeColor := color.NRGBA{R: 108, G: 185, B: 68, A: 255} // Truck green
	dottedBorder := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		scale := float64(w) / 650.0
		t := int(2.0 * scale)
		if t < 2 {
			t = 2
		}
		dotSize := int(4.0 * scale)
		if dotSize < 4 {
			dotSize = 4
		}
		gapSize := int(4.0 * scale)
		if gapSize < 4 {
			gapSize = 4
		}
		period := dotSize + gapSize

		// Rounded corner radius in physical pixels
		r := 16.0 * scale
		fx, fy := float64(x), float64(y)
		fw, fh := float64(w), float64(h)

		// For each corner, check if the pixel is inside the corner square but
		// outside the rounded arc — if so, skip it (transparent).
		inCorner := func(cx, cy float64) bool {
			dx, dy := fx-cx, fy-cy
			return dx*dx+dy*dy > r*r
		}
		if fx < r && fy < r && inCorner(r, r) {
			return color.Transparent
		}
		if fx >= fw-r && fy < r && inCorner(fw-r, r) {
			return color.Transparent
		}
		if fx < r && fy >= fh-r && inCorner(r, fh-r) {
			return color.Transparent
		}
		if fx >= fw-r && fy >= fh-r && inCorner(fw-r, fh-r) {
			return color.Transparent
		}

		// Draw the dotted border only along the edges (within thickness t)
		onLeft := x < t
		onRight := x >= w-t
		onTop := y < t
		onBottom := y >= h-t

		if onLeft || onRight {
			if (y % period) < dotSize {
				return strokeColor
			}
		}
		if onTop || onBottom {
			if (x % period) < dotSize {
				return strokeColor
			}
		}
		return color.Transparent
	})
	dottedBorder.SetMinSize(fyne.NewSize(650, 400))

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
		dottedBorder,
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
