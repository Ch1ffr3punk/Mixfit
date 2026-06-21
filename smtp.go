package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"fyne.io/fyne/v2"
)

func (n *Mixfit) sendYAMNViaSMTP(message []byte, entryRemailer string) error {
	config := &n.unifiedConfig.PingerConfig
	fyne.Do(func() {
		n.updateStatus("SMTP", "Parsing message headers...")
	})
	lines := strings.Split(string(message), "\n")
	var toAddr string
	for _, line := range lines {
		if strings.HasPrefix(line, "To: ") {
			toAddr = strings.TrimPrefix(line, "To: ")
			break
		}
	}
	if toAddr == "" {
		return fmt.Errorf("no To: header found in message")
	}
	if entryRemailer == "" {
		for _, line := range lines {
			if strings.Contains(line, "X-Remailer-Chain:") || strings.Contains(line, "Chain:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					chainStr := strings.TrimSpace(parts[1])
					chainParts := strings.Split(chainStr, ",")
					if len(chainParts) > 0 {
						entryRemailer = strings.TrimSpace(chainParts[0])
					}
				}
				break
			}
		}
	}

	if entryRemailer == "" {
		return fmt.Errorf("could not determine first remailer in chain")
	}

	if n.pubring == nil {
		return fmt.Errorf("pubring not initialized")
	}

	remailer, err := n.pubring.Get(entryRemailer)
	if err != nil {
		return fmt.Errorf("remailer '%s' not found in pubring: %v", entryRemailer, err)
	}

	smtpToAddr := remailer.Address
	if smtpToAddr == "" {
		return fmt.Errorf("remailer '%s' has no address", entryRemailer)
	}

	fyne.Do(func() {
		n.updateStatus("SMTP", fmt.Sprintf("Sending to: %s (via %s)", toAddr, smtpToAddr))
	})

	serverAddr := fmt.Sprintf("%s:%d", config.SMTPServer, config.SMTPPort)

	fyne.Do(func() {
		n.updateStatus("SMTP", fmt.Sprintf("Connecting to %s...", serverAddr))
	})

	var dialer func() (net.Conn, error)

	if config.Proxy != "" {
		proxyAddr := config.Proxy
		if !strings.Contains(proxyAddr, ":") {
			proxyAddr = "127.0.0.1:" + proxyAddr
		}

		fyne.Do(func() {
			n.updateStatus("SMTP", fmt.Sprintf("Using proxy: %s", proxyAddr))
		})

		socksDialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return fmt.Errorf("SOCKS5 dialer failed: %w", err)
		}

		dialer = func() (net.Conn, error) {
			return socksDialer.Dial("tcp", serverAddr)
		}
	} else {
		dialer = func() (net.Conn, error) {
			return net.Dial("tcp", serverAddr)
		}
	}

	var client *smtp.Client
	var conn net.Conn

	fyne.Do(func() {
		n.updateStatus("SMTP", "Establishing connection...")
	})

	if config.SMTPPort == 465 {
		tlsConfig := &tls.Config{
			ServerName:         config.SMTPServer,
			InsecureSkipVerify: config.SMTPSkipVerify,
		}

		conn, err = dialer()
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}

		fyne.Do(func() {
			n.updateStatus("SMTP", "Starting TLS handshake...")
		})

		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return fmt.Errorf("TLS handshake failed: %w", err)
		}

		client, err = smtp.NewClient(tlsConn, config.SMTPServer)
		if err != nil {
			tlsConn.Close()
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}
	} else {
		conn, err = dialer()
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}

		client, err = smtp.NewClient(conn, config.SMTPServer)
		if err != nil {
			conn.Close()
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}

		if config.SMTPUseTLS {
			fyne.Do(func() {
				n.updateStatus("SMTP", "Starting STARTTLS...")
			})

			tlsConfig := &tls.Config{
				ServerName:         config.SMTPServer,
				InsecureSkipVerify: config.SMTPSkipVerify,
			}
			if err := client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return fmt.Errorf("STARTTLS failed: %w", err)
			}
		}
	}
	defer client.Quit()

	fyne.Do(func() {
		n.updateStatus("SMTP", "Authenticating...")
	})

	from := "mix@nowhere.invalid"
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	fyne.Do(func() {
		n.updateStatus("SMTP", fmt.Sprintf("Sending to remailer: %s", smtpToAddr))
	})

	if err := client.Rcpt(smtpToAddr); err != nil {
		return fmt.Errorf("RCPT TO failed for %s: %w", smtpToAddr, err)
	}

	fyne.Do(func() {
		n.updateStatus("SMTP", "Sending message data...")
	})

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}
	defer wc.Close()

	if _, err := wc.Write(message); err != nil {
		return fmt.Errorf("writing data failed: %w", err)
	}

	fyne.Do(func() {
		n.updateStatus("SMTP", "Message sent successfully!")
	})

	return nil
}

func (n *Mixfit) sendMail() {
	bodyText := n.textArea.Text
	toText := strings.TrimSpace(n.toEntry.Text)
	if toText == "" {
		n.updateStatus("Error", "To: field cannot be empty")
		return
	}
	if strings.Contains(toText, ",") {
		n.updateStatus("Error", "Only one recipient allowed. Please remove commas.")
		return
	}
	recipient := strings.TrimSpace(toText)
	if !isValidEmail(recipient) {
		n.updateStatus("Error", fmt.Sprintf("Invalid email: %s", recipient))
		return
	}

	if n.mixChain == "" {
		n.updateStatus("Error", "Please configure remailer chain in Chain field")
		return
	}

	if strings.TrimSpace(bodyText) == "" && n.currentAttachment == nil {
		n.updateStatus("Error", "Message body cannot be empty")
		return
	}

	if strings.TrimSpace(n.subjectEntry.Text) == "" {
		n.updateStatus("Error", "Subject cannot be empty")
		return
	}

	if n.pubring == nil {
		n.updateStatus("Error", "Please download stats and keys first (Stats icon)")
		return
	}

	n.updateStatus("Sending", "Encoding YAMN message...")
	go func() {
		messageStr, entryRemailer, err := n.buildMixMessage(bodyText, n.currentAttachment, n.mixChain)
		if err != nil {
			fyne.Do(func() {
				n.updateStatus("Error", fmt.Sprintf("Encoding: %v", err))
			})
			return
		}

		fyne.Do(func() {
			n.updateStatus("SMTP", "Preparing SMTP delivery...")
		})

		startTime := time.Now()
		err = n.sendYAMNViaSMTP([]byte(messageStr), entryRemailer)
		elapsed := time.Since(startTime)

		if err != nil {
			fyne.Do(func() {
				errMsg := err.Error()
				if len(errMsg) > 80 {
					errMsg = wrapText(errMsg, 80)
				}
				n.updateStatus("Error", fmt.Sprintf("SMTP failed after %v: %s", elapsed.Round(time.Second), errMsg))
			})
		} else {
			fyne.Do(func() {
				n.updateStatus("Complete", fmt.Sprintf("Sent in %v via %s", elapsed.Round(time.Second), n.unifiedConfig.PingerConfig.SMTPServer))
				n.currentAttachment = nil
			})
		}
	}()
}
