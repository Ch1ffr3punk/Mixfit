package main

import (
	"crypto/rand"
	"crypto/sha3"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (e esub) deriveKey() []byte {
	pepper := []byte("fixed-pepper-1234")
	return argon2.IDKey([]byte(e.key), pepper, 3, 64*1024, 4, 32)
}

func (e *esub) esubgen() string {
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	key := e.deriveKey()
	cipher, _ := chacha20.NewUnauthenticatedCipher(key, nonce)
	textHash := sha3.Sum256([]byte("text"))
	ciphertext := make([]byte, 12)
	cipher.XORKeyStream(ciphertext, textHash[:12])
	return hex.EncodeToString(append(nonce, ciphertext...))
}

func (e *esub) esubtest() bool {
	if len(e.subject) != 48 {
		return false
	}
	esubBytes, err := hex.DecodeString(e.subject)
	if err != nil || len(esubBytes) != 24 {
		return false
	}
	nonce := esubBytes[:12]
	receivedCiphertext := esubBytes[12:]
	key := e.deriveKey()
	cipher, _ := chacha20.NewUnauthenticatedCipher(key, nonce)
	textHash := sha3.Sum256([]byte("text"))
	expectedCiphertext := make([]byte, 12)
	cipher.XORKeyStream(expectedCiphertext, textHash[:12])
	return hex.EncodeToString(expectedCiphertext) == hex.EncodeToString(receivedCiphertext)
}

func (e esub) checkReplayCache(app Mixfit) bool {
	if app.db == nil {
		return false
	}
	app.dbMutex.RLock()
	defer app.dbMutex.RUnlock()
	if _, exists := app.replayCache[e.subject]; exists {
		return true
	}
	var count int
	_ = app.db.QueryRow("SELECT COUNT(*) FROM esubs WHERE esub_hex = ?", e.subject).Scan(&count)
	return count > 0
}

func (e *esub) addToReplayCache(app *Mixfit, articleID int, newsgroup string) error {
	if app.db == nil {
		return nil
	}
	if e.checkReplayCache(*app) {
		return nil
	}
	app.dbMutex.Lock()
	defer app.dbMutex.Unlock()
	_, err := app.db.Exec("INSERT INTO esubs (esub_hex, first_seen, article_id, newsgroup) VALUES (?, ?, ?, ?)",
		e.subject, time.Now().Format(time.RFC3339), articleID, newsgroup)
	if err != nil {
		return err
	}
	app.replayCache[e.subject] = true
	return nil
}

func (n *Mixfit) createEsub() {
	keyEntry := widget.NewPasswordEntry()
	keyEntry.SetPlaceHolder("Password")
	dialog.ShowForm("Generate Esub", "Generate", "Cancel", []*widget.FormItem{{Text: "Password", Widget: keyEntry}}, func(confirmed bool) {
		if !confirmed {
			return
		}
		key := strings.TrimSpace(keyEntry.Text)
		if key == "" {
			dialog.ShowError(errors.New("Password cannot be empty"), n.window)
			return
		}
		e := &esub{key: key}
		generated := e.esubgen()
		e.subject = generated
		if e.esubtest() {
			n.window.Clipboard().SetContent(generated)
			dialog.ShowInformation("", fmt.Sprintf("esub: %s\n\nCopied to clipboard!", generated), n.window)
			n.updateStatus("Esub", "Created")
		}
	}, n.window)
}
