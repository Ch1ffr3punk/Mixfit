package main

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func getHTTPClient(proxyAddr string) (*http.Client, error) {
	if proxyAddr == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	if !strings.Contains(proxyAddr, ":") {
		proxyAddr = "127.0.0.1:" + proxyAddr
	}
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}
	transport := &http.Transport{
		Dial: dialer.Dial,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

func downloadFileWithClient(url string, client *http.Client) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("downloaded file is empty")
	}
	return data, nil
}

func (n *Mixfit) downloadPingerFiles() ([]byte, []byte, error) {
	config := &n.unifiedConfig.PingerConfig
	if len(config.Pingers) == 0 {
		return nil, nil, fmt.Errorf("no pingers configured")
	}
	var pubringData []byte
	var mlist2Data []byte
	var pubringFound bool
	var mlist2Found bool
	var httpClient *http.Client
	var err error
	if config.Proxy != "" {
		httpClient, err = getHTTPClient(config.Proxy)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create HTTP client with proxy: %w", err)
		}
	} else {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	for _, pinger := range config.Pingers {
		if !pubringFound {
			if data, err := downloadFileWithClient(pinger.URLs.Pubring, httpClient); err == nil {
				pubringData = data
				pubringFound = true
			}
		}

		if !mlist2Found {
			if data, err := downloadFileWithClient(pinger.URLs.Mlist2, httpClient); err == nil {
				mlist2Data = data
				mlist2Found = true
			}
		}

		if pubringFound && mlist2Found {
			break
		}
	}

	if !pubringFound {
		return nil, nil, fmt.Errorf("failed to download pubring.mix from any pinger")
	}

	return pubringData, mlist2Data, nil
}

func (n *Mixfit) downloadStatsAndKeys() {
	n.updateStatus("Downloading", "Fetching pubring and stats...")
	go func() {
		pubringData, mlist2Data, err := n.downloadPingerFiles()
		if err != nil {
			fyne.Do(func() {
				n.updateStatus("Error", fmt.Sprintf("Download failed: %v", err))
			})
			return
		}
		pubring := &Pubring{
			remailers: make(map[string]*Remailer),
			stats:     make(map[string]*Stats),
		}
		if err := pubring.ImportPubring(pubringData); err != nil {
			fyne.Do(func() {
				n.updateStatus("Error", fmt.Sprintf("Failed to import pubring: %v", err))
			})
			return
		}
		if len(mlist2Data) > 0 {
			if err := pubring.ImportStats(mlist2Data); err != nil {
				fyne.Do(func() {
					n.updateStatus("Warning", fmt.Sprintf("Stats import failed: %v", err))
				})
			} else {
				n.haveStats = true
			}
		}

		n.pubring = pubring

		names := make([]string, 0, len(pubring.remailers))
		for name := range pubring.remailers {
			names = append(names, name)
		}
		sort.Strings(names)

		var statsText strings.Builder
		statsText.WriteString(fmt.Sprintf("%-4s %-15s %10s %8s\n", "Role", "Remailer", "Latent", "Uptime"))
		statsText.WriteString(fmt.Sprintf("%-4s %-15s %10s %8s\n", "----", "--------", "------", "------"))

		for _, name := range names {
			stat := pubring.GetStats(name)
			role := "E"
			if stat != nil && stat.Options != "" {
				role = getRemailerRole(stat.Options)
			}

			if stat != nil {
				statsText.WriteString(fmt.Sprintf("%-4s %-15s %10s %8s\n",
					role,
					name,
					stat.LatentStr,
					stat.UptimeStr))
			} else {
				statsText.WriteString(fmt.Sprintf("%-4s %-15s %10s\n", role, name, "(no stats)"))
			}
		}

		fyne.Do(func() {
			n.updateStatus("Complete", fmt.Sprintf("Loaded %d remailers", len(pubring.remailers)))
			n.showStatsPopup(statsText.String())
		})
	}()
}

func (n *Mixfit) showStatsPopup(stats string) {
	statsLabel := widget.NewLabel(stats)
	statsLabel.Wrapping = fyne.TextWrapOff
	statsLabel.TextStyle = fyne.TextStyle{Monospace: true}
	statsContainer := container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10),
		container.New(layout.NewGridLayoutWithColumns(1), statsLabel))
	scroll := container.NewScroll(statsContainer)
	scroll.SetMinSize(fyne.NewSize(400, 450))
	okButton := widget.NewButton("OK", func() {
		if overlays := n.window.Canvas().Overlays(); overlays.Top() != nil {
			overlays.Remove(overlays.Top())
		}
	})
	okButton.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewLabelWithStyle("Remailer Statistics", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		scroll,
		container.NewHBox(layout.NewSpacer(), okButton, layout.NewSpacer()),
	)

	dialog.ShowCustomWithoutButtons("", content, n.window)
}
