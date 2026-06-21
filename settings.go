package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (n *Mixfit) showUnifiedConfig() {
	localArticleDirEntry := widget.NewEntry()
	localArticleDirEntry.SetText(n.unifiedConfig.LocalArticleDir)
	nntpServerEntry := widget.NewEntry()
	nntpServerEntry.SetText(n.unifiedConfig.NNTPConfig.Server)
	nntpPortEntry := widget.NewEntry()
	nntpPortEntry.SetText(fmt.Sprintf("%d", n.unifiedConfig.NNTPConfig.Port))
	newsgroupEntry := widget.NewEntry()
	newsgroupEntry.SetText(n.unifiedConfig.NNTPConfig.Newsgroup)
	smtpServerEntry := widget.NewEntry()
	smtpServerEntry.SetText(n.unifiedConfig.PingerConfig.SMTPServer)
	smtpPortEntry := widget.NewEntry()
	smtpPortEntry.SetText(fmt.Sprintf("%d", n.unifiedConfig.PingerConfig.SMTPPort))
	proxyEntry := widget.NewEntry()
	proxyEntry.SetText(n.unifiedConfig.PingerConfig.Proxy)
	useTLSCheck := widget.NewCheck("Use TLS", func(checked bool) {
		n.unifiedConfig.PingerConfig.SMTPUseTLS = checked
	})
	useTLSCheck.SetChecked(n.unifiedConfig.PingerConfig.SMTPUseTLS)
	skipVerifyCheck := widget.NewCheck("Skip TLS Verify", func(checked bool) {
		n.unifiedConfig.PingerConfig.SMTPSkipVerify = checked
	})
	skipVerifyCheck.SetChecked(n.unifiedConfig.PingerConfig.SMTPSkipVerify)
	resetCacheBtn := widget.NewButton("Reset Esub Cache", func() {
		dialog.ShowConfirm("Reset Cache", "Delete all cached esubs?", func(confirmed bool) {
			if confirmed && n.db != nil {
				_, _ = n.db.Exec("DELETE FROM esubs")
				n.replayCache = make(map[string]bool)
				n.unifiedConfig.NNTPConfig.LastArticle = 0
				n.saveUnifiedConfig()
				dialog.ShowInformation("", "Cache cleared!", n.window)
			}
		}, n.window)
	})
	resetCacheBtn.Importance = widget.LowImportance

	formItems := []*widget.FormItem{
		{Text: "Local Article Dir", Widget: localArticleDirEntry},
		{Text: "", Widget: widget.NewSeparator()},
		{Text: "NNTP Server", Widget: nntpServerEntry},
		{Text: "NNTP Port", Widget: nntpPortEntry},
		{Text: "Newsgroup", Widget: newsgroupEntry},
		{Text: "", Widget: widget.NewSeparator()},
		{Text: "SMTP Server", Widget: smtpServerEntry},
		{Text: "SMTP Port", Widget: smtpPortEntry},
		{Text: "Proxy", Widget: proxyEntry},
		{Text: "", Widget: useTLSCheck},
		{Text: "", Widget: skipVerifyCheck},
		{Text: "", Widget: resetCacheBtn},
	}

	var configDialog *dialog.CustomDialog

	saveBtn := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), func() {
		var nntpPort int
		_, _ = fmt.Sscanf(nntpPortEntry.Text, "%d", &nntpPort)
		if nntpPort == 0 {
			nntpPort = 119
		}
		var smtpPort int
		_, _ = fmt.Sscanf(smtpPortEntry.Text, "%d", &smtpPort)
		if smtpPort == 0 {
			smtpPort = 587
		}

		n.unifiedConfig.LocalArticleDir = strings.TrimSpace(localArticleDirEntry.Text)
		n.unifiedConfig.NNTPConfig.Server = strings.TrimSpace(nntpServerEntry.Text)
		n.unifiedConfig.NNTPConfig.Port = nntpPort
		n.unifiedConfig.NNTPConfig.Newsgroup = strings.TrimSpace(newsgroupEntry.Text)
		n.unifiedConfig.PingerConfig.SMTPServer = strings.TrimSpace(smtpServerEntry.Text)
		n.unifiedConfig.PingerConfig.SMTPPort = smtpPort
		n.unifiedConfig.PingerConfig.Proxy = strings.TrimSpace(proxyEntry.Text)
		n.saveUnifiedConfig()

		n.updateStatus("Settings", "Saved")

		if configDialog != nil {
			configDialog.Hide()
		}

		dialog.ShowInformation("Success", "Configuration saved!", n.window)
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if configDialog != nil {
			configDialog.Hide()
		}
	})

	buttons := container.NewHBox(layout.NewSpacer(), cancelBtn, saveBtn, layout.NewSpacer())

	content := container.NewVBox(
		widget.NewLabelWithStyle("Configuration", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewForm(formItems...),
		widget.NewSeparator(),
		buttons,
	)

	configDialog = dialog.NewCustomWithoutButtons("", content, n.window)
	configDialog.Show()
}
