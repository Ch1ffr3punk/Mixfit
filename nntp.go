package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (n *Mixfit) connectToNNTP(config *NNTPConfig) (net.Conn, *bufio.Reader, error) {
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)
	var conn net.Conn
	var err error
	proxyAddr := n.unifiedConfig.PingerConfig.Proxy
	if proxyAddr != "" {
		if !strings.Contains(proxyAddr, ":") {
			proxyAddr = "127.0.0.1:" + proxyAddr
		}

		dialer, dialerErr := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if dialerErr != nil {
			return nil, nil, fmt.Errorf("SOCKS5 dialer error: %v", dialerErr)
		}
		conn, err = dialer.Dial("tcp", addr)
		if err != nil {
			return nil, nil, fmt.Errorf("proxy connection failed: %v", err)
		}
	} else {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return nil, nil, fmt.Errorf("direct connection failed: %v", err)
		}
	}

	reader := bufio.NewReader(conn)

	response, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	if !strings.HasPrefix(response, "200 ") && !strings.HasPrefix(response, "201 ") {
		conn.Close()
		return nil, nil, fmt.Errorf("NNTP server error: %s", response)
	}

	return conn, reader, nil
}

func (n *Mixfit) fetchArticlesFromNewsgroup() {
	config := &n.unifiedConfig.NNTPConfig
	if config.Server == "" || config.Newsgroup == "" {
		dialog.ShowError(errors.New("Please configure NNTP server and Newsgroup in Settings"), n.window)
		return
	}
	keyEntry := widget.NewPasswordEntry()
	keyEntry.SetPlaceHolder("Password")
	items := []*widget.FormItem{
		{Text: "Password", Widget: keyEntry},
	}
	dialog.ShowForm("Fetch Articles", "Start", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}

		key := strings.TrimSpace(keyEntry.Text)
		if key == "" {
			dialog.ShowError(errors.New("Password cannot be empty"), n.window)
			return
		}

		n.progressContainer.Show()
		n.progressBar.SetValue(0)
		n.progressLabel.SetText("0%")
		n.updateStatus("Fetch", "Connecting to NNTP server...")

		go func() {
			err := n.processNewsgroup(config, key)
			fyne.Do(func() {
				if err != nil {
					if errors.Is(err, ErrNoNewArticles) {
						n.updateStatus("Complete", "No new articles to fetch")
						n.progressContainer.Hide()
					} else {
						n.updateStatus("Error", fmt.Sprintf("%v", err))
						n.progressContainer.Hide()
					}
				} else {
					n.updateStatus("Complete", fmt.Sprintf("Articles saved to %s", n.unifiedConfig.LocalArticleDir))
					n.progressBar.SetValue(1)
					n.progressLabel.SetText("100%")
					time.AfterFunc(3*time.Second, func() {
						fyne.Do(func() {
							n.progressContainer.Hide()
						})
					})
				}
			})
		}()
	}, n.window)
}

func (n *Mixfit) processNewsgroup(config *NNTPConfig, key string) error {
	conn, reader, err := n.connectToNNTP(config)
	if err != nil {
		return err
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GROUP %s\r\n", config.Newsgroup)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	var articleCount, first, last int
	_, err = fmt.Sscanf(response, "211 %d %d %d", &articleCount, &first, &last)
	if err != nil {
		return fmt.Errorf("failed to parse GROUP response: %v", err)
	}
	if first == 0 || last == 0 {
		return fmt.Errorf("no articles in Newsgroup")
	}
	startArticle := first

	if config.LastArticle > 0 {
		if config.LastArticle >= last {
			return ErrNoNewArticles
		}
		if config.LastArticle >= first {
			startArticle = config.LastArticle + 1
			fyne.Do(func() {
				n.updateStatus("Fetch", fmt.Sprintf("Resuming from article %d", startArticle))
			})
			time.Sleep(2 * time.Second)
		}
	}

	totalArticles := last - startArticle + 1
	if totalArticles <= 0 {
		return ErrNoNewArticles
	}

	current := 0

	fyne.Do(func() {
		n.updateStatus("Fetch", fmt.Sprintf("Fetching %d articles...", totalArticles))
		n.progressBar.SetValue(0)
		n.progressLabel.SetText("0%")
	})

	articlesDir := n.unifiedConfig.LocalArticleDir
	if articlesDir == "" {
		articlesDir = filepath.Join(getBaseDir(), "articles")
	}
	if err := os.MkdirAll(articlesDir, 0755); err != nil {
		return err
	}

	foundCount := 0
	maxProcessed := startArticle - 1

	for msgID := startArticle; msgID <= last; msgID++ {
		current++
		percent := float64(current) / float64(totalArticles)

		if msgID > maxProcessed {
			maxProcessed = msgID
		}

		fyne.Do(func() {
			n.progressBar.SetValue(percent)
			n.progressLabel.SetText(fmt.Sprintf("%d%%", int(percent*100)))
			n.updateStatus("Fetch", fmt.Sprintf("Article %d/%d", msgID, last))
		})

		fmt.Fprintf(conn, "ARTICLE %d\r\n", msgID)
		response, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		if !strings.HasPrefix(response, "220 ") {
			continue
		}

		var article strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			if line == ".\r\n" {
				break
			}
			if strings.HasPrefix(line, "..") {
				line = line[1:]
			}
			article.WriteString(line)
		}

		articleStr := article.String()
		var subject string

		lines := strings.Split(articleStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Subject: ") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					subject = strings.TrimSpace(parts[1])
					break
				}
			}
			if strings.HasPrefix(line, "X-Esub: ") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					subject = strings.TrimSpace(parts[1])
					break
				}
			}
		}

		if subject != "" && len(subject) == 48 {
			e := &esub{key: key, subject: subject}
			if e.esubtest() {
				if e.checkReplayCache(*n) {
					continue
				}

				outputFileName := filepath.Join(articlesDir, fmt.Sprintf("article_%d_%s.txt", msgID, e.subject))
				outputFile, err := os.Create(outputFileName)
				if err != nil {
					return err
				}

				outputFile.WriteString(fmt.Sprintf("Article-ID: %d\n", msgID))
				outputFile.WriteString(fmt.Sprintf("esub: %s\n", e.subject))
				outputFile.WriteString("---\n")
				outputFile.WriteString(articleStr)
				outputFile.Close()

				e.addToReplayCache(n, msgID, config.Newsgroup)
				foundCount++

				fyne.Do(func() {
					n.updateStatus("Fetch", fmt.Sprintf("Found %d esub(s)", foundCount))
				})
			}
		}

		time.Sleep(100 * time.Millisecond)

		if msgID%100 == 0 {
			config.LastArticle = maxProcessed
			n.saveUnifiedConfig()
		}
	}

	if maxProcessed > startArticle-1 {
		config.LastArticle = maxProcessed
		n.saveUnifiedConfig()
	}

	if foundCount == 0 && startArticle == first {
		return fmt.Errorf("No valid esub(s) found in %s", config.Newsgroup)
	} else if foundCount == 0 && startArticle > first {
		return fmt.Errorf("No new valid esub(s) found (last: %d)", startArticle-1)
	}

	return nil
}
