package main

import (
	"crypto/rand"
	"encoding/base64"
	"mime"
	"regexp"
	"strings"
)

func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

func encodeMIMESubject(input string) string {
	if input == "" {
		return ""
	}
	encoded := mime.BEncoding.Encode("UTF-8", input)
	parts := strings.Split(encoded, "?=")
	if len(parts) <= 1 {
		return encoded
	}
	var result string
	for i, part := range parts[:len(parts)-1] {
		if i > 0 {
			result += " "
		}
		result += part + "?=\n"
	}
	result += parts[len(parts)-1]
	return strings.TrimSuffix(result, "\n")
}

func wrapText(text string, maxLineLength int) string {
	if len(text) <= maxLineLength {
		return text
	}
	var result strings.Builder
	words := strings.Fields(text)
	currentLineLength := 0
	maxLines := 3
	lineCount := 1
	for i, word := range words {
		if lineCount >= maxLines {
			result.WriteString("...")
			break
		}
		if i > 0 {
			word = " " + word
		}
		if currentLineLength+len(word) > maxLineLength && currentLineLength > 0 {
			result.WriteString("\n")
			currentLineLength = 0
			lineCount++
			if lineCount >= maxLines {
				result.WriteString(word)
				result.WriteString("...")
				break
			}
		}
		result.WriteString(word)
		currentLineLength += len(word)
	}
	return result.String()
}

func (n *Mixfit) generateRandomBoundary() (string, error) {
	const boundaryChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	boundary := make([]byte, 24)
	for i := range boundary {
		randomByte := make([]byte, 1)
		_, err := rand.Read(randomByte)
		if err != nil {
			return "", err
		}
		boundary[i] = boundaryChars[int(randomByte[0])%len(boundaryChars)]
	}
	return string(boundary), nil
}

func (n *Mixfit) createMIMETextPart(plainText string) string {
	var textPart strings.Builder
	textPart.WriteString("Content-Type: text/plain; charset=UTF-8\n")
	textPart.WriteString("Content-Transfer-Encoding: 8bit\n")
	textPart.WriteString("\n")
	if plainText != "" {
		textPart.WriteString(plainText + "\n")
	} else {
		textPart.WriteString("(No text message)\n")
	}
	textPart.WriteString("\n")
	return textPart.String()
}

func (n *Mixfit) createMIMEAttachmentPart(attachment *Attachment) (string, error) {
	if attachment == nil {
		return "", nil
	}
	var attachmentPart strings.Builder
	attachmentPart.WriteString("Content-Type: " + attachment.ContentType + "; name=\"" + attachment.Filename + "\"\n")
	attachmentPart.WriteString("Content-Disposition: attachment; filename=\"" + attachment.Filename + "\"\n")
	attachmentPart.WriteString("Content-Transfer-Encoding: base64\n")
	attachmentPart.WriteString("\n")
	encoded := base64.StdEncoding.EncodeToString(attachment.Data)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		attachmentPart.WriteString(encoded[i:end] + "\n")
	}
	attachmentPart.WriteString("\n")
	return attachmentPart.String(), nil
}
