package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"image/color"
	"io"
	"log"
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
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.EqualFold(part, "__MACOSX") {
			return true
		}
		if strings.HasPrefix(part, ".") {
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

type customTheme struct{}

func (c customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 99, G: 102, B: 241, A: 255} // Modern Indigo
	}
	if name == theme.ColorNameFocus {
		return color.NRGBA{R: 139, G: 92, B: 246, A: 255} // Violet
	}
	if name == theme.ColorNameSelection {
		return color.NRGBA{R: 99, G: 102, B: 241, A: 80} // Soft selection overlay
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
		return color.NRGBA{R: 226, G: 232, B: 240, A: 255} // Slate-200
	}
	if name == theme.ColorNameInputBackground {
		if variant == theme.VariantDark {
			return color.NRGBA{R: 30, G: 41, B: 59, A: 255} // Slate-800
		}
		return color.NRGBA{R: 241, G: 245, B: 249, A: 255} // Slate-100
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (c customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (c customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (c customTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
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
	a.Settings().SetTheme(customTheme{})

	w := a.NewWindow("Outlook MSG to CSV Parser")
	w.Resize(fyne.NewSize(950, 700))

	var currentSources []msgSource
	var displayNames []string
	var csvData [][]string
	var updateSource func(path string)
	// Status Bar
	statusBar := widget.NewLabel("Feed me your msg files, Will")
	statusBar.TextStyle = fyne.TextStyle{Italic: true}

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
			label := box.Objects[0].(*widget.Label)
			label.SetText(displayNames[id])
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
					bg.FillColor = color.NRGBA{R: 224, G: 231, B: 255, A: 255}
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

	// Dialogs
	fileOpenDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		updateSource(path)
	}, w)
	fileOpenDialog.SetFilter(storage.NewExtensionFileFilter([]string{".msg", ".zip"}))

	folderOpenDialog := dialog.NewFolderOpen(func(reader fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if reader == nil {
			return
		}
		path := reader.Path()
		updateSource(path)
	}, w)

	bodyContainer := container.NewStack()

	var showWelcome func()
	var showWorkspace func()

	updateSource = func(path string) {
		sources, err := findMsgFiles(path)
		if err != nil {
			dialog.ShowError(err, w)
			statusBar.SetText("Error scanning source.")
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
			statusBar.SetText(fmt.Sprintf("Source scanned. Ready to parse %d files.", len(sources)))
			showWorkspace()
		} else {
			runButton.Disable()
			statusBar.SetText("No .msg files found in selected source.")
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
		statusBar.SetText("Parsing .msg files... Please wait.")

		go func() {
			records, err := parseMsgSources(currentSources)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, w)
					statusBar.SetText("Error parsing files.")
					runButton.Enable()
				})
				return
			}

			if len(records) == 0 {
				fyne.Do(func() {
					dialog.ShowInformation("No Data", "No structured records found in selected files.", w)
					statusBar.SetText("No records parsed.")
					runButton.Enable()
				})
				return
			}

			headers := []string{
				"source_file",
				"subject",
				"message_date",
				"reported_at",
				"dispatcher",
				"row_in_message",
				"raw_entry",
				"location_hint",
				"issue_type",
				"issue_time",
			}

			csvRows := [][]string{headers}
			for _, rec := range records {
				csvRows = append(csvRows, []string{
					rec.SourceFile,
					rec.Subject,
					rec.MessageDate,
					rec.ReportedAt,
					rec.Dispatcher,
					fmt.Sprintf("%d", rec.RowInMessage),
					rec.RawEntry,
					rec.LocationHint,
					rec.IssueType,
					rec.IssueTime,
				})
			}

			fyne.Do(func() {
				csvData = csvRows
				previewTable.Refresh()
				downloadButton.Enable()
				saveAsButton.Enable()
				statusBar.SetText(fmt.Sprintf("Successfully parsed %d records from %d files!", len(records), len(currentSources)))
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
			statusBar.SetText("Failed to save CSV automatically.")
			return
		}
		defer file.Close()

		csvWriter := csv.NewWriter(file)
		err = csvWriter.WriteAll(csvData)
		if err != nil {
			dialog.ShowError(err, w)
			statusBar.SetText("Failed to save CSV automatically.")
			return
		}
		statusBar.SetText(fmt.Sprintf("CSV saved automatically to %s", filename))

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
		fileSaveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			csvWriter := csv.NewWriter(writer)
			err = csvWriter.WriteAll(csvData)
			if err != nil {
				dialog.ShowError(err, w)
				statusBar.SetText("Failed to save CSV.")
				return
			}
			statusBar.SetText("CSV saved successfully.")
		}, w)
		datetime := time.Now().Format("2006-01-02_150405")
		fileSaveDialog.SetFileName(fmt.Sprintf("msg_parsed_%s.csv", datetime))
		fileSaveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
		fileSaveDialog.Show()
	}

	headerBg := canvas.NewRectangle(color.NRGBA{R: 79, G: 70, B: 229, A: 255})
	headerBg.SetMinSize(fyne.NewSize(0, 50))

	headerTitle := canvas.NewText("  will-msg 📬 Outlook MSG Parser", color.White)
	headerTitle.TextSize = 18
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}

	headerSubtitle := canvas.NewText("Feed me your msg files, Will", color.NRGBA{R: 199, G: 210, B: 254, A: 255})
	headerSubtitle.TextSize = 13
	headerSubtitle.TextStyle = fyne.TextStyle{Italic: true}

	headerContent := container.NewHBox(
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

	headline := canvas.NewText("Feed me your msg files, Will", color.NRGBA{R: 79, G: 70, B: 229, A: 255})
	headline.TextSize = 28
	headline.TextStyle = fyne.TextStyle{Bold: true}
	headline.Alignment = fyne.TextAlignCenter

	description := widget.NewLabel("Drop files, folders, or .zip archives here, or click to select.")
	description.Alignment = fyne.TextAlignCenter
	description.TextStyle = fyne.TextStyle{Italic: true}

	welcomeSelectFileBtn := widget.NewButtonWithIcon("Select MSG / ZIP File", theme.DocumentIcon(), func() {
		fileOpenDialog.Show()
	})
	welcomeSelectFileBtn.Importance = widget.HighImportance

	welcomeSelectFolderBtn := widget.NewButtonWithIcon("Select Folder", theme.FolderOpenIcon(), func() {
		folderOpenDialog.Show()
	})
	welcomeSelectFolderBtn.Importance = widget.HighImportance

	welcomeButtons := container.NewHBox(
		layout.NewSpacer(),
		welcomeSelectFileBtn,
		welcomeSelectFolderBtn,
		layout.NewSpacer(),
	)

	var dropZoneBg *canvas.Rectangle
	if a.Settings().ThemeVariant() == theme.VariantDark {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 30, G: 41, B: 59, A: 255})
	} else {
		dropZoneBg = canvas.NewRectangle(color.NRGBA{R: 238, G: 242, B: 255, A: 255})
	}
	dropZoneBg.SetMinSize(fyne.NewSize(650, 400))

	strokeColor := color.NRGBA{R: 99, G: 102, B: 241, A: 255}
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

		if x < t || x >= w-t {
			if (y % period) < dotSize {
				return strokeColor
			}
		}
		if y < t || y >= h-t {
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
		statusBar,
		nil,
		nil,
		bodyContainer,
	)

	w.SetContent(mainLayout)
	w.ShowAndRun()
}
