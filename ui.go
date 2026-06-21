package main

import (
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func NewFocusAwareEntry() *FocusAwareEntry {
	entry := &FocusAwareEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

func (e *FocusAwareEntry) FocusGained() {
	e.Entry.FocusGained()
	if e.onFocusChanged != nil {
		e.onFocusChanged(true)
	}
}

func (e *FocusAwareEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusChanged != nil {
		e.onFocusChanged(false)
	}
}

func (e *FocusAwareEntry) SetOnFocusChanged(callback func(bool)) {
	e.onFocusChanged = callback
}

func NewFocusAwareMultiLineEntry() *FocusAwareMultiLineEntry {
	entry := &FocusAwareMultiLineEntry{}
	entry.MultiLine = true
	entry.Wrapping = fyne.TextWrapOff
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.ExtendBaseWidget(entry)
	return entry
}

func (e *FocusAwareMultiLineEntry) FocusGained() {
	e.Entry.FocusGained()
	if e.onFocusChanged != nil {
		e.onFocusChanged(true)
	}
}

func (e *FocusAwareMultiLineEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusChanged != nil {
		e.onFocusChanged(false)
	}
}

func (e *FocusAwareMultiLineEntry) SetOnFocusChanged(callback func(bool)) {
	e.onFocusChanged = callback
}

func (g *oliveThemeWrapper) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (g *oliveThemeWrapper) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 128, G: 128, B: 0, A: 255}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 128, G: 128, B: 0, A: 255}
	default:
		return g.base.Color(name, variant)
	}
}

func (g *oliveThemeWrapper) Icon(name fyne.ThemeIconName) fyne.Resource {
	return g.base.Icon(name)
}

func (g *oliveThemeWrapper) Size(name fyne.ThemeSizeName) float32 {
	return g.base.Size(name)
}

func (n *Mixfit) toggleTheme() {
	fyne.Do(func() {
		if n.isDarkTheme {
			n.app.Settings().SetTheme(theme.LightTheme())
			n.isDarkTheme = false
			n.themeSwitch.SetText("🌙")
		} else {
			n.app.Settings().SetTheme(theme.DarkTheme())
			n.isDarkTheme = true
			n.themeSwitch.SetText("☀️")
		}
		n.app.Settings().SetTheme(&oliveThemeWrapper{base: n.app.Settings().Theme()})
		n.window.Content().Refresh()
	})
}

func (n *Mixfit) getMaxLabelWidth() float32 {
	labels := []string{"To:", "Subject:", "References:", "Followup-To:", "Newsgroups:", "Chain:"}
	var maxWidth float32 = 0
	for _, labelText := range labels {
		tmpLabel := widget.NewLabel(labelText)
		tmpLabel.TextStyle = fyne.TextStyle{Bold: true}
		if width := tmpLabel.MinSize().Width; width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth + 5
}

func (n *Mixfit) createCompactField(labelText string, entry fyne.CanvasObject) fyne.CanvasObject {
	labelWidget := widget.NewLabel(labelText + ":")
	labelWidget.TextStyle = fyne.TextStyle{Bold: true}
	labelWidget.Alignment = fyne.TextAlignTrailing
	maxLabelWidth := n.getMaxLabelWidth()
	return container.NewBorder(nil, nil, container.New(&fixedWidthLayout{width: maxLabelWidth}, labelWidget), nil, entry)
}

func (f *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) > 0 {
		objects[0].Resize(fyne.NewSize(f.width, objects[0].MinSize().Height))
		objects[0].Move(fyne.NewPos(0, 0))
	}
}

func (f *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) > 0 {
		return fyne.NewSize(f.width, objects[0].MinSize().Height)
	}
	return fyne.NewSize(f.width, 0)
}

func (n *Mixfit) getAdaptivePadding() float32 {
	scale := fyne.CurrentApp().Settings().Scale()
	padding := float32(8) * scale
	if padding < 6 {
		return 6
	}
	if padding > 16 {
		return 16
	}
	return padding
}

func NewFileFilter(name string, patterns ...string) *fileFilter {
	return &fileFilter{name: name, patterns: patterns}
}

func (f *fileFilter) Matches(uri fyne.URI) bool {
	ext := strings.ToLower(filepath.Ext(uri.Path()))
	for _, pattern := range f.patterns {
		if ext == pattern {
			return true
		}
	}
	return false
}

func (n *Mixfit) updateStatus(status string, detail string) {
	fyne.Do(func() {
		if n.statusResetTimer != nil {
			n.statusResetTimer.Stop()
			n.statusResetTimer = nil
		}
		n.statusLabel.SetText(status)
		if len(detail) > 70 {
			detail = wrapText(detail, 70)
		}
		n.statusDetail.SetText(detail)
		if status == "Complete" || status == "Error" {
			n.statusResetTimer = time.AfterFunc(8*time.Second, func() {
				fyne.Do(func() {
					if n.statusLabel.Text == status {
						n.statusLabel.SetText("Ready")
						n.statusDetail.SetText("Mixfit ready")
					}
					n.statusResetTimer = nil
				})
			})
		}
	})
}

func (n *Mixfit) resetStatus() {
	fyne.Do(func() {
		if n.statusLabel != nil {
			n.statusLabel.SetText("Ready")
		}
		if n.statusDetail != nil {
			n.statusDetail.SetText("Mixfit ready")
		}
	})
}

func (n *Mixfit) periodicStatusReset() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if n.window != nil {
				n.resetStatus()
			} else {
				return
			}
		}
	}()
}

func (n *Mixfit) hideNonEssentialUI() {
	if !n.bottomVisible {
		return
	}
	fyne.Do(func() {
		if n.mainScroll != nil {
			n.mainScroll.Content = container.NewBorder(n.headerSection, nil, nil, nil, n.textArea)
			n.mainScroll.Refresh()
			n.bottomVisible = false
		}
	})
}

func (n *Mixfit) showAllUI() {
	if n.bottomVisible {
		return
	}
	fyne.Do(func() {
		if n.mainScroll != nil {
			n.mainScroll.Content = container.NewBorder(n.headerSection, n.bottomSection, nil, nil, n.textArea)
			n.mainScroll.Refresh()
			n.bottomVisible = true
		}
	})
}

func (n *Mixfit) setupFocusHandlers() {
	focusHandler := func(focused bool) {
		if focused {
			if n.hideTimer != nil {
				n.hideTimer.Stop()
				n.hideTimer = nil
			}
			n.hideNonEssentialUI()
		} else {
			if n.hideTimer != nil {
				n.hideTimer.Stop()
			}
			n.hideTimer = time.AfterFunc(500*time.Millisecond, func() {
				fyne.Do(func() { n.showAllUI() })
			})
		}
	}
	n.toEntry.SetOnFocusChanged(focusHandler)
	n.subjectEntry.SetOnFocusChanged(focusHandler)
	n.referencesEntry.SetOnFocusChanged(focusHandler)
	n.followupToEntry.SetOnFocusChanged(focusHandler)
	n.newsgroupsEntry.SetOnFocusChanged(focusHandler)
	n.textArea.SetOnFocusChanged(focusHandler)
}
