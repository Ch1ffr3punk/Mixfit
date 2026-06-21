package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (n *Mixfit) attachImageHandler() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			n.updateStatus("Error", fmt.Sprintf("File error: %v", err))
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		imageData, err := io.ReadAll(reader)
		if err != nil {
			n.updateStatus("Error", fmt.Sprintf("Error reading file: %v", err))
			return
		}
		if len(imageData) < 768 {
			n.updateStatus("Error", fmt.Sprintf("Image too small: %d bytes (min. 768 bytes)", len(imageData)))
			return
		}
		if len(imageData) > 2700 {
			n.updateStatus("Error", fmt.Sprintf("Image too large: %d bytes (max. 2700 bytes)", len(imageData)))
			return
		}
		contentType := "image/jpeg"
		if len(imageData) >= 8 && imageData[0] == 0x89 && imageData[1] == 0x50 &&
			imageData[2] == 0x4E && imageData[3] == 0x47 {
			contentType = "image/png"
		}
		originalFilename := filepath.Base(reader.URI().Path())

		n.currentAttachment = &Attachment{
			Data:        imageData,
			Filename:    originalFilename,
			ContentType: contentType,
		}

		n.updateStatus("Complete", fmt.Sprintf("Image ready: %s (%d bytes). Press Send to attach.", originalFilename, len(imageData)))
	}, n.window)

	fd.SetFilter(NewFileFilter("Image files", ".png", ".jpg", ".jpeg"))
	fd.Show()
}

func (n *Mixfit) viewArticle() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, n.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		content, err := io.ReadAll(reader)
		if err != nil {
			dialog.ShowError(fmt.Errorf("error reading file: %v", err), n.window)
			return
		}
		articleStr := string(content)
		separator := "---\n"
		sepIdx := strings.Index(articleStr, separator)
		if sepIdx != -1 {
			articleStr = articleStr[sepIdx+len(separator):]
		}
		images, err := decodeMultipartImages(articleStr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("error decoding: %v", err), n.window)
			return
		}
		if len(images) == 0 {
			dialog.ShowInformation("No Images", "No AEC images found in article.", n.window)
			return
		}

		n.showImageDialog(images[0], len(images))
	}, n.window)
}

func decodeMultipartImages(article string) ([][]byte, error) {
	var images [][]byte
	boundary := ""
	lines := strings.Split(article, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Content-Type: multipart/") {
			if idx := strings.Index(line, "boundary="); idx != -1 {
				boundaryPart := line[idx+9:]
				boundary = strings.Trim(boundaryPart, "\"")
				break
			}
		}
	}
	if boundary == "" {
		for _, line := range lines {
			if strings.HasPrefix(line, "----=_Part") {
				boundary = strings.TrimSpace(line)
				break
			}
		}
	}
	if boundary == "" {
		return nil, errors.New("multipart boundary not found")
	}
	parts := strings.Split(article, boundary)
	for _, part := range parts {
		if strings.Contains(part, "Content-Type: image/png") {
			imageData, err := extractPNGFromPart(part)
			if err == nil && len(imageData) > 0 {
				images = append(images, imageData)
			}
		}
	}

	return images, nil
}

func extractPNGFromPart(part string) ([]byte, error) {
	lines := strings.Split(part, "\n")
	var base64Lines []string
	inData := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "Content-Transfer-Encoding: base64") {
			inData = true
			continue
		}
		if inData && line == "" {
			continue
		}
		if inData && !strings.HasPrefix(line, "Content-") && line != "" && !strings.HasPrefix(line, "--") {
			base64Lines = append(base64Lines, line)
		}
		if inData && strings.HasPrefix(line, "Content-Type:") {
			break
		}
	}
	if len(base64Lines) == 0 {
		return nil, errors.New("no base64 data found")
	}
	base64Str := strings.Join(base64Lines, "")
	return base64.StdEncoding.DecodeString(base64Str)
}

func (n *Mixfit) showImageDialog(imageData []byte, totalImages int) {
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		dialog.ShowError(fmt.Errorf("error decoding PNG: %v", err), n.window)
		return
	}
	bounds := img.Bounds()
	scaledImg := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		srcY := y * bounds.Dy() / 512
		for x := 0; x < 512; x++ {
			srcX := x * bounds.Dx() / 512
			scaledImg.Set(x, y, img.At(srcX, srcY))
		}
	}
	fyneImg := canvas.NewImageFromImage(scaledImg)
	fyneImg.FillMode = canvas.ImageFillContain
	fyneImg.SetMinSize(fyne.NewSize(512, 512))

	infoText := "AEC Image"
	if totalImages > 1 {
		infoText = fmt.Sprintf("AEC Image 1 of %d", totalImages)
	}

	imageWindow := n.app.NewWindow("Article Image")

	closeBtn := widget.NewButton("Close", func() {
		imageWindow.Close()
	})
	closeBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewLabel(infoText),
		fyneImg,
		container.NewCenter(closeBtn),
	)

	imageWindow.Resize(fyne.NewSize(600, 700))
	imageWindow.SetContent(content)
	imageWindow.CenterOnScreen()
	imageWindow.Show()
}
