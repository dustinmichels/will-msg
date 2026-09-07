package main

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"
)

var activeRuleEditorWindow fyne.Window

// RuleManagerView coordinates the rule editor UI, persistence, and live sandbox.
type RuleManagerView struct {
	window        fyne.Window
	app           fyne.App
	workingConfig RuleConfig
	configPath    string
	selectedIndex int

	table          *widget.Table
	ruleCountLabel *widget.Label

	// Toolbar action buttons
	addBtn      *widget.Button
	editBtn     *widget.Button
	deleteBtn   *widget.Button
	moveUpBtn   *widget.Button
	moveDownBtn *widget.Button
	toggleBtn   *widget.Button
	resetBtn    *widget.Button
	importBtn   *widget.Button
	exportBtn   *widget.Button

	// Configuration settings
	heuristicsCheck    *widget.Check
	defaultLabelSelect *widget.Select

	// Live Sandbox widgets
	sandboxInput        *widget.Entry
	sandboxAddressLabel *widget.Label
	sandboxStatusLabel  *widget.Label
	sandboxMatchedRule  *widget.Label
	sandboxLabelLabel   *widget.Label
	sandboxMetricLabel  *widget.Label
	sandboxTimeLabel    *widget.Label

	onApplied func()
}

// ShowRuleEditorWindow opens or brings to focus the Rule & Label Manager window.
func ShowRuleEditorWindow(a fyne.App, parent fyne.Window, onApplied func()) fyne.Window {
	if activeRuleEditorWindow != nil {
		activeRuleEditorWindow.RequestFocus()
		return activeRuleEditorWindow
	}

	w := a.NewWindow("Rules & Labels Manager")
	w.Resize(fyne.NewSize(980, 720))
	w.CenterOnScreen()

	view := NewRuleManagerView(a, w, onApplied)
	w.SetContent(view.BuildUI())

	w.SetOnClosed(func() {
		activeRuleEditorWindow = nil
	})

	activeRuleEditorWindow = w
	w.Show()
	return w
}

// NewRuleManagerView creates and initializes a RuleManagerView with active configuration.
func NewRuleManagerView(a fyne.App, w fyne.Window, onApplied func()) *RuleManagerView {
	return NewRuleManagerViewWithPath(a, w, "", onApplied)
}

// NewRuleManagerViewWithPath creates a RuleManagerView with a specific configuration file path.
func NewRuleManagerViewWithPath(a fyne.App, w fyne.Window, configPath string, onApplied func()) *RuleManagerView {
	var cfg RuleConfig
	if configPath != "" {
		loaded, err := LoadConfigFromPath(configPath)
		if err != nil {
			loaded = DefaultRuleConfig()
		}
		cfg = loaded.Clone()
	} else {
		cfg = LoadConfig().Clone()
	}
	view := &RuleManagerView{
		window:        w,
		app:           a,
		workingConfig: cfg,
		configPath:    configPath,
		selectedIndex: -1,
		onApplied:     onApplied,
	}
	return view
}

// BuildUI constructs the complete CanvasObject layout for the rule manager window.
func (v *RuleManagerView) BuildUI() fyne.CanvasObject {
	// Top Header Banner
	headerBg := canvas.NewRectangle(color.NRGBA{R: 108, G: 185, B: 68, A: 255}) // Truck green
	headerBg.SetMinSize(fyne.NewSize(0, 48))

	headerTitle := canvas.NewText("  Rules & Labels Manager", color.White)
	headerTitle.TextSize = 17
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}

	headerSubtitle := canvas.NewText("Customize classification patterns, precedence, labels, and heuristics", color.NRGBA{R: 220, G: 245, B: 195, A: 255})
	headerSubtitle.TextSize = 12
	headerSubtitle.TextStyle = fyne.TextStyle{Italic: true}

	headerContent := container.NewHBox(
		widget.NewIcon(theme.SettingsIcon()),
		headerTitle,
		layout.NewSpacer(),
		headerSubtitle,
		canvas.NewText("   ", color.Transparent),
	)
	header := container.NewStack(headerBg, container.NewPadded(headerContent))

	// Toolbar Buttons
	v.addBtn = widget.NewButtonWithIcon("Add Rule", theme.ContentAddIcon(), func() {
		v.showAddRuleDialog()
	})
	v.addBtn.Importance = widget.HighImportance

	v.editBtn = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if v.selectedIndex >= 0 && v.selectedIndex < len(v.workingConfig.Rules) {
			v.showEditRuleDialog(v.selectedIndex)
		}
	})
	v.editBtn.Disable()

	v.deleteBtn = widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		v.showDeleteRuleConfirm()
	})
	v.deleteBtn.Disable()

	v.moveUpBtn = widget.NewButtonWithIcon("Move Up", theme.MoveUpIcon(), func() {
		v.moveSelectedRule(-1)
	})
	v.moveUpBtn.Disable()

	v.moveDownBtn = widget.NewButtonWithIcon("Move Down", theme.MoveDownIcon(), func() {
		v.moveSelectedRule(1)
	})
	v.moveDownBtn.Disable()

	v.toggleBtn = widget.NewButtonWithIcon("Toggle On/Off", theme.ViewRefreshIcon(), func() {
		v.toggleSelectedRuleEnabled()
	})
	v.toggleBtn.Disable()

	v.resetBtn = widget.NewButtonWithIcon("Reset Defaults", theme.ViewRefreshIcon(), func() {
		v.showResetConfirm()
	})

	v.importBtn = widget.NewButtonWithIcon("Import JSON", theme.FolderOpenIcon(), func() {
		v.importRules()
	})

	v.exportBtn = widget.NewButtonWithIcon("Export JSON", theme.DocumentSaveIcon(), func() {
		v.exportRules()
	})

	toolbarLeft := container.NewHBox(
		v.addBtn,
		v.editBtn,
		v.deleteBtn,
		v.moveUpBtn,
		v.moveDownBtn,
		v.toggleBtn,
	)

	toolbarRight := container.NewHBox(
		v.resetBtn,
		v.importBtn,
		v.exportBtn,
	)

	toolbar := container.NewHBox(
		toolbarLeft,
		layout.NewSpacer(),
		toolbarRight,
	)

	// Settings & Heuristics Bar
	v.heuristicsCheck = widget.NewCheck("Enable heuristic fallback matching (e.g. *_not_out)", func(checked bool) {
		v.workingConfig.EnableHeuristics = checked
		v.runSandbox()
	})
	v.heuristicsCheck.SetChecked(v.workingConfig.EnableHeuristics)

	labelOptions := v.getLabelKeys()
	v.defaultLabelSelect = widget.NewSelect(labelOptions, func(s string) {
		v.workingConfig.DefaultLabel = s
		v.runSandbox()
	})
	v.defaultLabelSelect.SetSelected(v.workingConfig.DefaultLabel)

	v.ruleCountLabel = widget.NewLabel(fmt.Sprintf("Total Rules: %d", len(v.workingConfig.Rules)))
	v.ruleCountLabel.TextStyle = fyne.TextStyle{Bold: true}

	settingsBar := container.NewHBox(
		v.heuristicsCheck,
		layout.NewSpacer(),
		widget.NewLabel("Fallback Default Label:"),
		v.defaultLabelSelect,
		canvas.NewText("    ", color.Transparent),
		v.ruleCountLabel,
	)

	// Rules Table
	v.table = widget.NewTable(
		func() (int, int) {
			return len(v.workingConfig.Rules) + 1, 7
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

			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				if v.app.Settings().ThemeVariant() == theme.VariantDark {
					bg.FillColor = color.NRGBA{R: 30, G: 41, B: 59, A: 255}
				} else {
					bg.FillColor = color.NRGBA{R: 220, G: 245, B: 195, A: 255}
				}
				switch id.Col {
				case 0:
					label.SetText("#")
				case 1:
					label.SetText("Type")
				case 2:
					label.SetText("Pattern")
				case 3:
					label.SetText("Target Label")
				case 4:
					label.SetText("Metric")
				case 5:
					label.SetText("Enabled")
				case 6:
					label.SetText("Description")
				}
			} else {
				ruleIdx := id.Row - 1
				if ruleIdx >= len(v.workingConfig.Rules) {
					return
				}
				rule := v.workingConfig.Rules[ruleIdx]

				if ruleIdx == v.selectedIndex {
					bg.FillColor = color.NRGBA{R: 108, G: 185, B: 68, A: 80}
					label.TextStyle = fyne.TextStyle{Bold: true}
				} else {
					bg.FillColor = color.Transparent
					if !rule.Enabled {
						label.TextStyle = fyne.TextStyle{Italic: true}
					} else {
						label.TextStyle = fyne.TextStyle{}
					}
				}

				switch id.Col {
				case 0:
					label.SetText(fmt.Sprintf("%d", id.Row))
				case 1:
					label.SetText(string(rule.Type))
				case 2:
					label.SetText(rule.Pattern)
				case 3:
					label.SetText(rule.Label)
				case 4:
					label.SetText(string(v.workingConfig.MetricForLabel(rule.Label)))
				case 5:
					if rule.Enabled {
						label.SetText("✓ Yes")
					} else {
						label.SetText("✗ No")
					}
				case 6:
					label.SetText(rule.Description)
				}
			}
			label.Refresh()
			bg.Refresh()
		},
	)

	v.table.SetColumnWidth(0, 45)  // #
	v.table.SetColumnWidth(1, 95)  // Type
	v.table.SetColumnWidth(2, 260) // Pattern
	v.table.SetColumnWidth(3, 190) // Target Label
	v.table.SetColumnWidth(4, 95)  // Metric
	v.table.SetColumnWidth(5, 75)  // Enabled
	v.table.SetColumnWidth(6, 200) // Description

	v.table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 {
			v.selectedIndex = id.Row - 1
			v.updateToolbarButtons()
			v.table.Refresh()
		}
	}

	// Live Sandbox Panel
	sandboxCard := v.buildSandboxUI()

	// Bottom Action Bar
	saveApplyBtn := widget.NewButtonWithIcon("Save & Apply Rules", theme.DocumentSaveIcon(), func() {
		v.saveAndApply()
	})
	saveApplyBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if v.window != nil {
			v.window.Close()
		}
	})

	bottomBar := container.NewHBox(
		widget.NewLabel("Changes take effect immediately upon saving."),
		layout.NewSpacer(),
		cancelBtn,
		saveApplyBtn,
	)

	topSection := container.NewVBox(
		header,
		container.NewPadded(toolbar),
		container.NewPadded(settingsBar),
	)

	split := container.NewVSplit(v.table, sandboxCard)
	split.Offset = 0.58

	mainLayout := container.NewBorder(
		topSection,
		container.NewPadded(bottomBar),
		nil,
		nil,
		container.NewPadded(split),
	)

	v.runSandbox()
	return mainLayout
}

// buildSandboxUI constructs the live rule testing sandbox card.
func (v *RuleManagerView) buildSandboxUI() fyne.CanvasObject {
	sandboxTitle := widget.NewLabelWithStyle("Live Rule Tester / Sandbox", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sandboxHelp := widget.NewLabelWithStyle("Type or paste any raw message line to verify address splitting, matching rule precedence, and assigned label in real time:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	v.sandboxInput = widget.NewEntry()
	v.sandboxInput.SetPlaceHolder("e.g. 45 FOREST ST TRASH AND RCY NOT OUT 0830AM")
	v.sandboxInput.SetText("45 FOREST ST TRASH AND RCY NOT OUT 0830AM")
	v.sandboxInput.OnChanged = func(s string) {
		v.runSandbox()
	}

	sample1 := widget.NewButton("Sample: Both Not Out", func() {
		v.sandboxInput.SetText("45 FOREST ST TRASH AND RCY NOT OUT 0830AM")
	})
	sample2 := widget.NewButton("Sample: Contaminated", func() {
		v.sandboxInput.SetText("14 DARTMOUTH ST RECYC CONTAM")
	})
	sample3 := widget.NewButton("Sample: Blocked", func() {
		v.sandboxInput.SetText("88 BOSTON AVE BLOCKED ACCESS")
	})
	sample4 := widget.NewButton("Sample: Heuristic Suffix", func() {
		v.sandboxInput.SetText("10 HIGH ST MATTRESS NOT OUT")
	})
	clearBtn := widget.NewButton("Clear", func() {
		v.sandboxInput.SetText("")
	})

	sampleBar := container.NewHBox(
		widget.NewLabel("Quick Samples:"),
		sample1,
		sample2,
		sample3,
		sample4,
		layout.NewSpacer(),
		clearBtn,
	)

	v.sandboxAddressLabel = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.sandboxStatusLabel = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.sandboxMatchedRule = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.sandboxLabelLabel = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.sandboxMetricLabel = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.sandboxTimeLabel = widget.NewLabelWithStyle("-", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	resultsGrid := container.NewGridWithColumns(3,
		container.NewVBox(widget.NewLabel("Detected Address:"), v.sandboxAddressLabel),
		container.NewVBox(widget.NewLabel("Detected Issue/Status:"), v.sandboxStatusLabel),
		container.NewVBox(widget.NewLabel("Issue Time:"), v.sandboxTimeLabel),
		container.NewVBox(widget.NewLabel("Assigned Label:"), v.sandboxLabelLabel),
		container.NewVBox(widget.NewLabel("Matched Rule:"), v.sandboxMatchedRule),
		container.NewVBox(widget.NewLabel("Metric Impact:"), v.sandboxMetricLabel),
	)

	sandboxContent := container.NewVBox(
		sandboxTitle,
		sandboxHelp,
		v.sandboxInput,
		sampleBar,
		canvas.NewText(" ", color.Transparent),
		resultsGrid,
	)

	return container.NewPadded(sandboxContent)
}

// runSandbox runs classification on the current sandbox input using the workingConfig.
func (v *RuleManagerView) runSandbox() {
	if v.sandboxInput == nil || v.sandboxAddressLabel == nil {
		return
	}

	text := strings.TrimSpace(v.sandboxInput.Text)
	if text == "" {
		v.sandboxAddressLabel.SetText("-")
		v.sandboxStatusLabel.SetText("-")
		v.sandboxTimeLabel.SetText("-")
		v.sandboxLabelLabel.SetText("-")
		v.sandboxMatchedRule.SetText("(Enter text above)")
		v.sandboxMetricLabel.SetText("-")
		return
	}

	tempEngine := NewRuleEngine(v.workingConfig)
	loc, issue, label, issueTime := tempEngine.Classify(text)
	rule, ruleIdx, matched := tempEngine.MatchRule(issue)
	metric := tempEngine.MetricForLabel(label)

	if loc != "" {
		v.sandboxAddressLabel.SetText(loc)
	} else {
		v.sandboxAddressLabel.SetText("(none detected)")
	}

	if issue != "" {
		v.sandboxStatusLabel.SetText(issue)
	} else {
		v.sandboxStatusLabel.SetText("(none detected)")
	}

	if issueTime != "" {
		v.sandboxTimeLabel.SetText(issueTime)
	} else {
		v.sandboxTimeLabel.SetText("(none)")
	}

	v.sandboxLabelLabel.SetText(label)

	if matched {
		v.sandboxMatchedRule.SetText(fmt.Sprintf("Rule #%d: %s (%s)", ruleIdx+1, rule.Pattern, rule.Type))
	} else if v.workingConfig.EnableHeuristics && strings.HasSuffix(strings.ToLower(strings.ReplaceAll(issue, " ", "_")), "_not_out") {
		v.sandboxMatchedRule.SetText("Dynamic Heuristic (*_not_out)")
	} else {
		v.sandboxMatchedRule.SetText(fmt.Sprintf("Default Fallback (%s)", v.workingConfig.DefaultLabel))
	}

	switch metric {
	case MetricTrash:
		v.sandboxMetricLabel.SetText("Trash")
	case MetricRecycling:
		v.sandboxMetricLabel.SetText("Recycling")
	case MetricBoth:
		v.sandboxMetricLabel.SetText("Both (Trash + Recycling)")
	default:
		v.sandboxMetricLabel.SetText("None")
	}
}

// updateToolbarButtons updates enabled/disabled states of toolbar buttons.
func (v *RuleManagerView) updateToolbarButtons() {
	hasSelection := v.selectedIndex >= 0 && v.selectedIndex < len(v.workingConfig.Rules)
	if !hasSelection {
		v.editBtn.Disable()
		v.deleteBtn.Disable()
		v.moveUpBtn.Disable()
		v.moveDownBtn.Disable()
		v.toggleBtn.Disable()
		return
	}

	v.editBtn.Enable()
	v.deleteBtn.Enable()
	v.toggleBtn.Enable()

	if v.selectedIndex > 0 {
		v.moveUpBtn.Enable()
	} else {
		v.moveUpBtn.Disable()
	}

	if v.selectedIndex < len(v.workingConfig.Rules)-1 {
		v.moveDownBtn.Enable()
	} else {
		v.moveDownBtn.Disable()
	}
}

// refreshUI refreshes the table, settings, count label, buttons, and sandbox.
func (v *RuleManagerView) refreshUI() {
	if v.ruleCountLabel != nil {
		v.ruleCountLabel.SetText(fmt.Sprintf("Total Rules: %d", len(v.workingConfig.Rules)))
	}
	if v.defaultLabelSelect != nil {
		opts := v.getLabelKeys()
		v.defaultLabelSelect.Options = opts
		v.defaultLabelSelect.SetSelected(v.workingConfig.DefaultLabel)
		v.defaultLabelSelect.Refresh()
	}
	if v.heuristicsCheck != nil {
		v.heuristicsCheck.SetChecked(v.workingConfig.EnableHeuristics)
	}
	if v.table != nil {
		v.table.Refresh()
	}
	v.updateToolbarButtons()
	v.runSandbox()
}

// moveSelectedRule shifts the selected rule by delta (-1 for up, +1 for down).
func (v *RuleManagerView) moveSelectedRule(delta int) {
	if v.selectedIndex < 0 || v.selectedIndex >= len(v.workingConfig.Rules) {
		return
	}
	target := v.selectedIndex + delta
	if target < 0 || target >= len(v.workingConfig.Rules) {
		return
	}

	v.workingConfig.Rules[v.selectedIndex], v.workingConfig.Rules[target] = v.workingConfig.Rules[target], v.workingConfig.Rules[v.selectedIndex]
	v.selectedIndex = target
	v.refreshUI()
}

// toggleSelectedRuleEnabled flips the Enabled flag of the selected rule.
func (v *RuleManagerView) toggleSelectedRuleEnabled() {
	if v.selectedIndex < 0 || v.selectedIndex >= len(v.workingConfig.Rules) {
		return
	}
	v.workingConfig.Rules[v.selectedIndex].Enabled = !v.workingConfig.Rules[v.selectedIndex].Enabled
	v.refreshUI()
}

// showDeleteRuleConfirm prompts the user to confirm deleting the selected rule.
func (v *RuleManagerView) showDeleteRuleConfirm() {
	if v.selectedIndex < 0 || v.selectedIndex >= len(v.workingConfig.Rules) {
		return
	}
	rule := v.workingConfig.Rules[v.selectedIndex]
	dialog.ShowConfirm(
		"Delete Rule",
		fmt.Sprintf("Are you sure you want to delete rule #%d (%s)?", v.selectedIndex+1, rule.Pattern),
		func(confirmed bool) {
			if confirmed {
				v.workingConfig.Rules = append(v.workingConfig.Rules[:v.selectedIndex], v.workingConfig.Rules[v.selectedIndex+1:]...)
				if v.selectedIndex >= len(v.workingConfig.Rules) {
					v.selectedIndex = len(v.workingConfig.Rules) - 1
				}
				v.refreshUI()
			}
		},
		v.window,
	)
}

// showResetConfirm prompts the user to confirm resetting rules to built-in defaults.
func (v *RuleManagerView) showResetConfirm() {
	dialog.ShowConfirm(
		"Reset to Built-in Defaults",
		"Are you sure you want to reset all classification rules and labels to built-in defaults?\nAny custom rules will be replaced.",
		func(confirmed bool) {
			if confirmed {
				v.workingConfig = DefaultRuleConfig()
				v.selectedIndex = -1
				v.refreshUI()
			}
		},
		v.window,
	)
}

// getLabelKeys returns all label keys defined in workingConfig.
func (v *RuleManagerView) getLabelKeys() []string {
	keys := make([]string, 0, len(v.workingConfig.Labels))
	for _, l := range v.workingConfig.Labels {
		keys = append(keys, l.Key)
	}
	return keys
}

// showAddRuleDialog displays the modal dialog to create a new rule.
func (v *RuleManagerView) showAddRuleDialog() {
	v.showRuleFormDialog(nil, -1)
}

// showEditRuleDialog displays the modal dialog to edit an existing rule.
func (v *RuleManagerView) showEditRuleDialog(index int) {
	if index < 0 || index >= len(v.workingConfig.Rules) {
		return
	}
	rule := v.workingConfig.Rules[index]
	v.showRuleFormDialog(&rule, index)
}

// showRuleFormDialog opens a custom dialog to add or edit a ClassificationRule.
func (v *RuleManagerView) showRuleFormDialog(initialRule *ClassificationRule, editIndex int) {
	isNew := initialRule == nil

	title := "Add Classification Rule"
	if !isNew {
		title = fmt.Sprintf("Edit Rule #%d", editIndex+1)
	}

	patternEntry := widget.NewEntry()
	patternEntry.SetPlaceHolder("e.g. MSW AND RECYC NOT OUT or \\bBLOCKED\\b")

	typeSelect := widget.NewSelect([]string{"substring", "regex"}, nil)
	typeSelect.SetSelected("substring")

	labelSelect := widget.NewSelect(append(v.getLabelKeys(), "[+ Add New Label...]"), nil)

	customKeyEntry := widget.NewEntry()
	customKeyEntry.SetPlaceHolder("e.g. yard_waste_not_out")

	customDisplayNameEntry := widget.NewEntry()
	customDisplayNameEntry.SetPlaceHolder("e.g. Yard Waste Not Out")

	customMetricSelect := widget.NewSelect([]string{"none", "trash", "recycling", "both"}, nil)
	customMetricSelect.SetSelected("none")

	customLabelBox := container.NewVBox(
		widget.NewLabelWithStyle("New Label Definition:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Label Key (snake_case):", customKeyEntry),
			widget.NewFormItem("Display Name:", customDisplayNameEntry),
			widget.NewFormItem("Metric Contribution:", customMetricSelect),
		),
	)
	customLabelBox.Hide()

	labelSelect.OnChanged = func(s string) {
		if s == "[+ Add New Label...]" {
			customLabelBox.Show()
		} else {
			customLabelBox.Hide()
		}
	}

	descriptionEntry := widget.NewEntry()
	descriptionEntry.SetPlaceHolder("Optional rule notes")

	enabledCheck := widget.NewCheck("Rule Enabled", nil)
	enabledCheck.SetChecked(true)

	if !isNew {
		patternEntry.SetText(initialRule.Pattern)
		typeSelect.SetSelected(string(initialRule.Type))
		labelSelect.SetSelected(initialRule.Label)
		descriptionEntry.SetText(initialRule.Description)
		enabledCheck.SetChecked(initialRule.Enabled)
	} else {
		if len(v.workingConfig.Labels) > 0 {
			labelSelect.SetSelected(v.workingConfig.Labels[0].Key)
		}
	}

	form := widget.NewForm(
		widget.NewFormItem("Pattern:", patternEntry),
		widget.NewFormItem("Match Type:", typeSelect),
		widget.NewFormItem("Target Label:", labelSelect),
		widget.NewFormItem("Description:", descriptionEntry),
		widget.NewFormItem("Status:", enabledCheck),
	)

	dialogContainer := container.NewVBox(
		form,
		customLabelBox,
	)

	var d dialog.Dialog
	d = dialog.NewCustomConfirm(
		title,
		"Save Rule",
		"Cancel",
		dialogContainer,
		func(save bool) {
			if !save {
				return
			}

			pat := strings.TrimSpace(patternEntry.Text)
			if pat == "" {
				dialog.ShowError(errors.New("rule pattern cannot be empty"), v.window)
				return
			}

			ruleType := RuleType(typeSelect.Selected)
			if ruleType == RuleTypeSubstring {
				pat = strings.ToUpper(pat)
			} else if ruleType == RuleTypeRegex {
				if _, err := compileRegexPattern(pat); err != nil {
					dialog.ShowError(fmt.Errorf("invalid regex pattern: %w", err), v.window)
					return
				}
			}

			targetLabel := labelSelect.Selected
			if targetLabel == "[+ Add New Label...]" {
				key := strings.TrimSpace(customKeyEntry.Text)
				displayName := strings.TrimSpace(customDisplayNameEntry.Text)
				metric := MetricContribution(customMetricSelect.Selected)

				if key == "" {
					dialog.ShowError(errors.New("new label key cannot be empty"), v.window)
					return
				}
				if displayName == "" {
					displayName = key
				}

				// Check for collision or update
				existingIdx := -1
				for i, l := range v.workingConfig.Labels {
					if l.Key == key {
						existingIdx = i
						break
					}
				}
				if existingIdx >= 0 {
					v.workingConfig.Labels[existingIdx] = LabelDefinition{
						Key:         key,
						DisplayName: displayName,
						Metric:      metric,
					}
				} else {
					v.workingConfig.Labels = append(v.workingConfig.Labels, LabelDefinition{
						Key:         key,
						DisplayName: displayName,
						Metric:      metric,
					})
				}
				targetLabel = key
			}

			if targetLabel == "" {
				dialog.ShowError(errors.New("target label must be selected"), v.window)
				return
			}

			ruleID := ""
			if !isNew {
				ruleID = initialRule.ID
			} else {
				ruleID = fmt.Sprintf("%s_%d", strings.ToLower(targetLabel), time.Now().UnixNano())
			}

			newRule := ClassificationRule{
				ID:          ruleID,
				Pattern:     pat,
				Type:        ruleType,
				Label:       targetLabel,
				Description: strings.TrimSpace(descriptionEntry.Text),
				Enabled:     enabledCheck.Checked,
			}

			if isNew {
				v.workingConfig.Rules = append(v.workingConfig.Rules, newRule)
				v.selectedIndex = len(v.workingConfig.Rules) - 1
			} else {
				v.workingConfig.Rules[editIndex] = newRule
			}

			v.refreshUI()
		},
		v.window,
	)
	d.Resize(fyne.NewSize(540, 400))
	d.Show()
}

// ApplyChanges validates the configuration, saves it to disk, and updates the active engine.
func (v *RuleManagerView) ApplyChanges() error {
	if err := v.workingConfig.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if v.configPath != "" {
		if err := SaveConfigToPath(v.configPath, v.workingConfig); err != nil {
			return fmt.Errorf("failed to save rules to disk: %w", err)
		}
	} else {
		if err := SaveConfig(v.workingConfig); err != nil {
			return fmt.Errorf("failed to save rules to disk: %w", err)
		}
	}
	SetDefaultEngine(NewRuleEngine(v.workingConfig))

	if v.onApplied != nil {
		v.onApplied()
	}
	return nil
}

// saveAndApply validates, persists to disk, updates the in-memory engine, and closes the window.
func (v *RuleManagerView) saveAndApply() {
	if err := v.ApplyChanges(); err != nil {
		if v.window != nil {
			dialog.ShowError(err, v.window)
		}
		return
	}

	if v.window != nil {
		dialog.ShowInformation("Rules Saved & Applied", "Classification rules have been saved and applied to the parser.", v.window)
		v.window.Close()
	}
}

// exportRules opens a file picker and exports the workingConfig to JSON.
func (v *RuleManagerView) exportRules() {
	go func() {
		path, err := zenity.SelectFileSave(
			zenity.Title("Export Classification Rules"),
			zenity.Filename("will_msg_rules.json"),
			zenity.FileFilters{
				{Name: "JSON files", Patterns: []string{"*.json"}},
			},
		)
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				fyne.Do(func() {
					dialog.ShowError(err, v.window)
				})
			}
			return
		}

		if err := ExportConfigFile(path, v.workingConfig); err != nil {
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("failed to export rules: %w", err), v.window)
			})
			return
		}

		fyne.Do(func() {
			dialog.ShowInformation("Export Succeeded", fmt.Sprintf("Rules successfully exported to:\n%s", path), v.window)
		})
	}()
}

// importRules opens a file picker, imports JSON, validates it, and updates workingConfig.
func (v *RuleManagerView) importRules() {
	go func() {
		path, err := zenity.SelectFile(
			zenity.Title("Import Classification Rules"),
			zenity.FileFilters{
				{Name: "JSON files", Patterns: []string{"*.json"}},
			},
		)
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				fyne.Do(func() {
					dialog.ShowError(err, v.window)
				})
			}
			return
		}

		imported, err := ImportConfigFile(path)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("failed to import rules: %w", err), v.window)
			})
			return
		}

		fyne.Do(func() {
			v.workingConfig = imported
			v.selectedIndex = -1
			v.refreshUI()
			dialog.ShowInformation("Import Succeeded", fmt.Sprintf("Successfully imported %d rules from:\n%s", len(imported.Rules), path), v.window)
		})
	}()
}
