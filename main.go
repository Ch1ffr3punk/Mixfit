package main

import (
	"log"
	"net/url"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (n *Mixfit) showInfoPopup() {
	projURL, _ := url.Parse("https://github.com/Ch1ffr3punk/Mixfit")
	projectLink := widget.NewHyperlink("An Open Source project", projURL)
	okButton := widget.NewButton("OK", func() {
		if overlays := n.window.Canvas().Overlays(); overlays.Top() != nil {
			overlays.Remove(overlays.Top())
		}
	})
	okButton.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewLabelWithStyle("Mixfit YAMN Client v0.1.0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), projectLink, layout.NewSpacer()),
		widget.NewLabelWithStyle("released under the MIT license", fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabelWithStyle("© 2014 Steve Crook", fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabelWithStyle("© 2026 Ch1ffr3punk", fyne.TextAlignCenter, fyne.TextStyle{}),
		container.NewHBox(layout.NewSpacer(), okButton, layout.NewSpacer()),
	)
	dialog.ShowCustomWithoutButtons("", content, n.window)
}

func (n *Mixfit) clearContent() {
	fyne.Do(func() {
		n.toEntry.SetText("")
		n.subjectEntry.SetText("")
		n.referencesEntry.SetText("")
		n.followupToEntry.SetText("")
		n.newsgroupsEntry.SetText("")
		n.textArea.SetText("")
		n.chainEntry.SetText("")
		n.currentAttachment = nil
		n.mixChain = ""
		if clipboard := n.window.Clipboard(); clipboard != nil {
			clipboard.SetContent("")
		}
		n.updateStatus("Ready", "All content cleared and clipboard emptied")
	})
}

func (n *Mixfit) setupResponsiveUI() fyne.CanvasObject {
	toEntry := NewFocusAwareEntry()
	toEntry.PlaceHolder = "Recipient"
	subjectEntry := NewFocusAwareEntry()
	subjectEntry.PlaceHolder = "Subject"
	referencesEntry := NewFocusAwareEntry()
	referencesEntry.PlaceHolder = "Message-ID (optional)"
	followupToEntry := NewFocusAwareEntry()
	followupToEntry.PlaceHolder = "Followup-To (optional)"
	newsgroupsEntry := NewFocusAwareEntry()
	newsgroupsEntry.PlaceHolder = "Newsgroups (optional)"
	textArea := NewFocusAwareMultiLineEntry()
	textArea.PlaceHolder = "Enter your message here..."
	chainEntry := NewFocusAwareEntry()
	chainEntry.PlaceHolder = "remailer1,remailer2,..."
	n.chainEntry = chainEntry
	n.toEntry = toEntry
	n.subjectEntry = subjectEntry
	n.referencesEntry = referencesEntry
	n.followupToEntry = followupToEntry
	n.newsgroupsEntry = newsgroupsEntry
	n.textArea = textArea
	n.statusLabel = widget.NewLabel("Ready")
	n.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	n.statusDetail = widget.NewLabel("Mixfit ready")
	statusBar := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			n.statusLabel,
			widget.NewLabel("|"),
			n.statusDetail,
			layout.NewSpacer(),
		),
	)

	n.themeSwitch = widget.NewButton("☀️", n.toggleTheme)
	n.configBtn = widget.NewButtonWithIcon("", theme.SettingsIcon(), n.showUnifiedConfig)
	n.infoBtn = widget.NewButtonWithIcon("", theme.InfoIcon(), n.showInfoPopup)

	statsBtn := widget.NewButtonWithIcon("", theme.ListIcon(), n.downloadStatsAndKeys)

	topBar := container.NewHBox(
		n.configBtn,
		layout.NewSpacer(),
		statsBtn,
		layout.NewSpacer(),
		n.infoBtn,
		layout.NewSpacer(),
		widget.NewButtonWithIcon("", theme.ContentClearIcon(), n.clearContent),
		layout.NewSpacer(),
		n.themeSwitch,
	)

	aecButton := widget.NewButton("AEC", n.attachImageHandler)
	aecButton.Importance = widget.HighImportance

	sendButton := widget.NewButton("Send", n.sendMail)
	sendButton.Importance = widget.HighImportance

	createEsubButton := widget.NewButton("Esub", n.createEsub)
	createEsubButton.Importance = widget.HighImportance

	fetchArticlesButton := widget.NewButton("Fetch", n.fetchArticlesFromNewsgroup)
	fetchArticlesButton.Importance = widget.HighImportance

	viewButton := widget.NewButton("View", n.viewArticle)
	viewButton.Importance = widget.HighImportance

	n.progressBar = widget.NewProgressBar()
	n.progressLabel = widget.NewLabel("0%")
	n.progressContainer = container.NewVBox(widget.NewLabel("Progress:"), n.progressBar, n.progressLabel)
	n.progressContainer.Hide()

	buttonContainer := container.New(layout.NewGridLayoutWithColumns(5), aecButton, sendButton, createEsubButton, fetchArticlesButton, viewButton)

	headerSection := container.NewVBox(
		topBar,
		widget.NewSeparator(),
		n.createCompactField("To", toEntry),
		n.createCompactField("Subject", subjectEntry),
		n.createCompactField("References", referencesEntry),
		n.createCompactField("Followup-To", followupToEntry),
		n.createCompactField("Newsgroups", newsgroupsEntry),
		n.createCompactField("Chain", chainEntry),
		widget.NewSeparator(),
	)
	bottomSection := container.NewVBox(buttonContainer, n.progressContainer, widget.NewSeparator(), statusBar)

	n.headerSection = headerSection
	n.bottomSection = bottomSection
	n.bottomVisible = true
	n.mixChain = ""

	chainEntry.OnChanged = func(s string) {
		n.mixChain = strings.TrimSpace(s)
	}

	textScroll := container.NewScroll(textArea)
	textScroll.Direction = container.ScrollBoth

	mainContent := container.NewBorder(headerSection, bottomSection, nil, nil, textScroll)
	n.mainScroll = container.NewScroll(mainContent)
	paddedContent := container.New(layout.NewCustomPaddedLayout(n.getAdaptivePadding(), n.getAdaptivePadding(), n.getAdaptivePadding(), n.getAdaptivePadding()), n.mainScroll)
	n.setupFocusHandlers()
	return paddedContent
}

func main() {
	if err := ensureBaseDir(); err != nil {
		log.Printf("Warning: Failed to create base directory: %v", err)
	}
	myApp := app.New()
	window := myApp.NewWindow("Mixfit")
	unifiedConfig, err := loadOrCreateUnifiedConfig()
	if err != nil {
		log.Printf("Config error: %v\n", err)
		os.Exit(1)
	}

	myApp.Settings().SetTheme(&oliveThemeWrapper{
		base: theme.DarkTheme(),
	})

	mixfitInstance := &Mixfit{
		app:           myApp,
		window:        window,
		isDarkTheme:   true,
		unifiedConfig: unifiedConfig,
		pool:          pool,
		articlesDir:   unifiedConfig.LocalArticleDir,
		pubring:       nil,
		haveStats:     false,
		mixChain:      "",
	}

	if err := mixfitInstance.initDB(); err != nil {
		log.Printf("Warning: Database initialization failed: %v\n", err)
	}

	window.SetContent(mixfitInstance.setupResponsiveUI())
	window.SetPadded(true)
	window.SetMaster()
	window.Resize(fyne.NewSize(640, 720))
	window.CenterOnScreen()
	mixfitInstance.periodicStatusReset()
	mixfitInstance.resetStatus()

	window.ShowAndRun()
}
