package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/crooks/yamn/crandom"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/nacl/box"
)

func aesCtr(in, key, iv []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	out := make([]byte, len(in))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(out, in)
	return out
}

func newSlotFinal() *slotFinal {
	return &slotFinal{
		aesIV:          crandom.Randbytes(16),
		chunkNum:       1,
		numChunks:      1,
		messageID:      crandom.Randbytes(16),
		packetID:       crandom.Randbytes(16),
		gotBodyBytes:   false,
		deliveryMethod: 0,
	}
}

func (f *slotFinal) setNumChunks(n int) {
	f.numChunks = uint8(n)
}

func (f *slotFinal) setBodyBytes(length int) {
	if length > bodyBytes {
		fmt.Fprintf(os.Stderr, "Error: body (%d bytes) exceeds maximum (%d bytes)\n", length, bodyBytes)
		os.Exit(1)
	}
	f.bodyBytes = length
	f.gotBodyBytes = true
}

func (f *slotFinal) encode() []byte {
	if !f.gotBodyBytes {
		fmt.Fprintln(os.Stderr, "Error: cannot encode slot final before body length is defined")
		os.Exit(1)
	}
	buf := new(bytes.Buffer)
	buf.Write(f.aesIV)
	buf.WriteByte(f.chunkNum)
	buf.WriteByte(f.numChunks)
	buf.Write(f.messageID)
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(f.bodyBytes))
	buf.Write(tmp)
	buf.WriteByte(f.deliveryMethod)
	if buf.Len() != 39 {
		fmt.Fprintf(os.Stderr, "Error: incorrect buffer length: Wanted=39, Got=%d\n", buf.Len())
		os.Exit(1)
	}
	buf.WriteString(strings.Repeat("\x00", encDataBytes-buf.Len()))
	return buf.Bytes()
}

func (f *slotFinal) getPacketID() []byte {
	return f.packetID
}

func newSlotData() *slotData {
	timestamp := make([]byte, 2)
	ts := time.Now().UTC().Unix() / 86400
	ts -= int64(crandom.Dice() % 4)
	binary.LittleEndian.PutUint16(timestamp, uint16(ts))
	return &slotData{
		version:       2,
		packetType:    0,
		protocol:      0,
		packetID:      crandom.Randbytes(16),
		gotAesKey:     false,
		aesKey:        make([]byte, 32),
		timestamp:     timestamp,
		gotPacketInfo: false,
		gotTagHash:    false,
		tagHash:       make([]byte, 32),
	}
}

func (head *slotData) setExit() {
	head.packetType = 1
}

func (head *slotData) setAesKey(key []byte) {
	if len(key) != 32 {
		fmt.Fprintf(os.Stderr, "Error: invalid key length. Expected=32, Got=%d\n", len(key))
		os.Exit(1)
	}
	copy(head.aesKey, key)
	head.gotAesKey = true
}

func (head *slotData) setPacketID(id []byte) {
	if len(id) != 16 {
		fmt.Fprintf(os.Stderr, "Error: invalid packet ID length. Expected=16, Got=%d\n", len(id))
		os.Exit(1)
	}
	copy(head.packetID, id)
}

func (head *slotData) setTagHash(hash []byte) {
	if len(hash) != 32 {
		fmt.Fprintf(os.Stderr, "Error: invalid hash length. Expected=32, Got=%d\n", len(hash))
		os.Exit(1)
	}
	copy(head.tagHash, hash)
	head.gotTagHash = true
}

func (head *slotData) setPacketInfo(ei []byte) {
	if len(ei) != encDataBytes {
		fmt.Fprintf(os.Stderr, "Error: invalid packet info length. Expected=%d, Got=%d\n", encDataBytes, len(ei))
		os.Exit(1)
	}
	head.gotPacketInfo = true
	head.packetInfo = ei
}

func (head *slotData) encode() []byte {
	if !head.gotAesKey {
		fmt.Fprintln(os.Stderr, "Error: AES key not specified")
		os.Exit(1)
	}
	if !head.gotPacketInfo {
		fmt.Fprintln(os.Stderr, "Error: packet info not defined")
		os.Exit(1)
	}
	if !head.gotTagHash {
		fmt.Fprintln(os.Stderr, "Error: anti-tag hash not defined")
		os.Exit(1)
	}
	buf := new(bytes.Buffer)
	buf.WriteByte(head.version)
	buf.WriteByte(head.packetType)
	buf.WriteByte(head.protocol)
	buf.Write(head.packetID)
	buf.Write(head.aesKey)
	buf.Write(head.timestamp)
	buf.Write(head.packetInfo)
	buf.Write(head.tagHash)
	if buf.Len() != 149 {
		fmt.Fprintf(os.Stderr, "Error: incorrect buffer length: Expected=149, Got=%d\n", buf.Len())
		os.Exit(1)
	}
	buf.WriteString(strings.Repeat("\x00", encHeadBytes-buf.Len()))
	return buf.Bytes()
}

func newEncodeHeader() *encodeHeader {
	return &encodeHeader{
		gotRecipient:   false,
		recipientKeyID: make([]byte, 16),
	}
}

func (h *encodeHeader) setRecipient(recipientKeyID, recipientPK []byte) {
	if len(recipientKeyID) != 16 {
		fmt.Fprintf(os.Stderr, "Error: invalid key ID length. Expected=16, Got=%d\n", len(recipientKeyID))
		os.Exit(1)
	}
	if len(recipientPK) != 32 {
		fmt.Fprintf(os.Stderr, "Error: invalid public key length. Expected=32, Got=%d\n", len(recipientPK))
		os.Exit(1)
	}
	copy(h.recipientPK[:], recipientPK)
	copy(h.recipientKeyID, recipientKeyID)
	h.gotRecipient = true
}

func (h *encodeHeader) encode(encHead []byte) []byte {
	if !h.gotRecipient {
		fmt.Fprintln(os.Stderr, "Error: header encode without defining recipient")
		os.Exit(1)
	}
	if len(encHead) != encHeadBytes {
		fmt.Fprintf(os.Stderr, "Error: invalid encrypted header length. Expected=%d, Got=%d\n", encHeadBytes, len(encHead))
		os.Exit(1)
	}
	senderPK, senderSK, err := box.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}
	var nonce [24]byte
	copy(nonce[:], crandom.Randbytes(24))
	buf := new(bytes.Buffer)
	buf.Write(h.recipientKeyID)
	buf.Write(senderPK[:])
	buf.Write(nonce[:])
	buf.Write(box.Seal(nil, encHead, &nonce, &h.recipientPK, senderSK))
	if buf.Len() != 248 {
		fmt.Fprintf(os.Stderr, "Error: incorrect buffer length: Expected=248, Got=%d\n", buf.Len())
		os.Exit(1)
	}
	buf.Write(crandom.Randbytes(headerBytes - buf.Len()))
	return buf.Bytes()
}

func newEncMessage() *encMessage {
	return &encMessage{
		gotPayload:  false,
		payload:     make([]byte, messageBytes),
		chainLength: 0,
	}
}

func (m *encMessage) getPayload() []byte {
	return m.payload
}

func (m *encMessage) setChainLength(chainLength int) {
	if chainLength > maxChainLength {
		fmt.Fprintf(os.Stderr, "Error: chain length (%d) exceeds maximum (%d)\n", chainLength, maxChainLength)
		os.Exit(1)
	}
	if chainLength <= 0 {
		fmt.Fprintln(os.Stderr, "Error: chain length cannot be negative or zero")
		os.Exit(1)
	}
	m.chainLength = chainLength
	m.intermediateHops = chainLength - 1
	m.padHeaders = maxChainLength - m.chainLength
	m.padBytes = m.padHeaders * headerBytes
	copy(m.payload, crandom.Randbytes(m.padBytes))
	for n := 0; n < m.intermediateHops; n++ {
		m.keys[n] = crandom.Randbytes(32)
		m.ivs[n] = crandom.Randbytes(12)
	}
}

func (m *encMessage) setPlainText(plain []byte) (plainLength int) {
	plainLength = len(plain)
	if plainLength > bodyBytes {
		fmt.Fprintf(os.Stderr, "Error: payload (%d) exceeds max length (%d)\n", plainLength, bodyBytes)
		os.Exit(1)
	}
	copy(m.payload[headersBytes:], plain)
	m.gotPayload = true
	return
}

func (m *encMessage) getIntermediateHops() int {
	if m.chainLength == 0 {
		fmt.Fprintln(os.Stderr, "Error: cannot get hop count. Chain length is not defined")
		os.Exit(1)
	}
	return m.intermediateHops
}

func (m *encMessage) getKey(intermediateHop int) []byte {
	if m.chainLength == 0 {
		fmt.Fprintln(os.Stderr, "Error: cannot get a Key until the chain length is defined")
		os.Exit(1)
	}
	if intermediateHop >= m.intermediateHops {
		fmt.Fprintf(os.Stderr, "Error: requested key for hop (%d) exceeds array length (%d)\n", intermediateHop, m.intermediateHops)
		os.Exit(1)
	}
	return m.keys[intermediateHop]
}

func (m *encMessage) getPartialIV(intermediateHop int) []byte {
	if intermediateHop > m.intermediateHops {
		fmt.Fprintf(os.Stderr, "Error: requested IV for hop (%d) exceeds array length (%d)\n", intermediateHop, m.intermediateHops)
		os.Exit(1)
	}
	return m.ivs[intermediateHop]
}

func (m *encMessage) getAntiTag() []byte {
	digest, err := blake2s.New256(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating digest: %v\n", err)
		os.Exit(1)
	}
	digest.Write(m.payload[headerBytes:])
	return digest.Sum(nil)
}

func (m *encMessage) encryptBody(key, iv []byte) {
	if !m.gotPayload {
		fmt.Fprintln(os.Stderr, "Error: cannot encrypt payload until it's defined")
		os.Exit(1)
	}
	if len(key) != 32 {
		fmt.Fprintf(os.Stderr, "Error: invalid key length. Expected=32, Got=%d\n", len(key))
		os.Exit(1)
	}
	if len(iv) != 16 {
		fmt.Fprintf(os.Stderr, "Error: invalid IV length. Expected=16, Got=%d\n", len(iv))
		os.Exit(1)
	}
	copy(
		m.payload[headersBytes:],
		aesCtr(
			m.payload[headersBytes:],
			key,
			iv,
		),
	)
}

func (m *encMessage) encryptAll(hop int) {
	key := m.getKey(hop)
	for slot := 0; slot < maxChainLength; slot++ {
		sbyte := slot * headerBytes
		ebyte := (slot + 1) * headerBytes
		iv := m.getIV(hop, slot)
		copy(
			m.payload[sbyte:ebyte],
			aesCtr(m.payload[sbyte:ebyte], key, iv),
		)
	}
	iv := m.getIV(hop, maxChainLength)
	copy(
		m.payload[headersBytes:],
		aesCtr(m.payload[headersBytes:], key, iv),
	)
}

func (m *encMessage) getIV(intermediateHop, slot int) []byte {
	if m.chainLength == 0 {
		fmt.Fprintln(os.Stderr, "Error: cannot get an IV until the chain length is defined")
		os.Exit(1)
	}
	return seqIV(m.ivs[intermediateHop], slot)
}

func seqIV(partialIV []byte, slot int) []byte {
	if len(partialIV) != 12 {
		fmt.Fprintf(os.Stderr, "Error: invalid iv input: expected 12 bytes, got %d bytes\n", len(partialIV))
		os.Exit(1)
	}
	iv := make([]byte, 16)
	copy(iv[0:4], partialIV[0:4])
	copy(iv[8:16], partialIV[4:12])
	ctr := make([]byte, 4)
	binary.LittleEndian.PutUint32(ctr, uint32(slot))
	copy(iv[4:8], ctr)
	return iv
}

func (m *encMessage) shiftHeaders() {
	bottomHeader := headersBytes - headerBytes
	copy(m.payload[headerBytes:], m.payload[:bottomHeader])
}

func (m *encMessage) insertHeader(header []byte) {
	if len(header) != headerBytes {
		fmt.Fprintf(os.Stderr, "Error: invalid header length. Expected=%d, Got=%d\n", headerBytes, len(header))
		os.Exit(1)
	}
	copy(m.payload[:headerBytes], header)
}

func (m *encMessage) deterministic(hop int) {
	if m.chainLength == 0 {
		fmt.Fprintln(os.Stderr, "Error: cannot generate deterministic headers until chain length has been specified")
		os.Exit(1)
	}
	bottomSlot := maxChainLength - 1
	topSlot := bottomSlot - (m.intermediateHops - hop - 1)
	for slot := topSlot; slot <= bottomSlot; slot++ {
		right := bottomSlot - slot + hop
		useSlot := bottomSlot
		fakeHead := make([]byte, headerBytes)
		for interHop := right; interHop-hop >= 0; interHop-- {
			key := m.getKey(interHop)
			iv := m.getIV(interHop, useSlot)
			copy(fakeHead, aesCtr(fakeHead, key, iv))
			useSlot--
		}
		sByte := slot * headerBytes
		eByte := sByte + headerBytes
		copy(m.payload[sByte:eByte], fakeHead)
	}
}

func newSlotIntermediate() *slotIntermediate {
	return &slotIntermediate{
		gotAesIV12: false,
		aesIV12:    make([]byte, 12),
		nextHop:    make([]byte, 52),
	}
}

func (s *slotIntermediate) setPartialIV(partialIV []byte) {
	if len(partialIV) != 12 {
		fmt.Fprintf(os.Stderr, "Error: invalid iv input: expected 12 bytes, got %d bytes\n", len(partialIV))
		os.Exit(1)
	}
	s.gotAesIV12 = true
	copy(s.aesIV12, partialIV)
}

func (s *slotIntermediate) setNextHop(nh string) {
	if len(nh) > 52 {
		fmt.Fprintln(os.Stderr, "Error: next hop address exceeds 52 chars")
		os.Exit(1)
	}
	s.nextHop = []byte(nh + strings.Repeat("\x00", 52-len(nh)))
}

func (s *slotIntermediate) encode() []byte {
	if !s.gotAesIV12 {
		fmt.Fprintln(os.Stderr, "Error: cannot encode until partial IV is defined")
		os.Exit(1)
	}
	buf := new(bytes.Buffer)
	buf.Write(s.aesIV12)
	buf.Write(s.nextHop)
	if buf.Len() != 64 {
		fmt.Fprintf(os.Stderr, "Error: incorrect buffer length: Expected=64, Got=%d\n", buf.Len())
		os.Exit(1)
	}
	return buf.Bytes()
}

func popstr(s *[]string) string {
	slice := *s
	if len(slice) == 0 {
		return ""
	}
	element := slice[len(slice)-1]
	slice = slice[:len(slice)-1]
	*s = slice
	return element
}

func wrap64(writer io.Writer, b []byte, wrap int) {
	breaker := NewLineBreaker(writer, wrap)
	b64 := base64.NewEncoder(base64.StdEncoding, breaker)
	b64.Write(b)
	b64.Close()
	breaker.Close()
}

func NewLineBreaker(out io.Writer, lineLength int) *lineBreaker {
	return &lineBreaker{
		lineLength:  lineLength,
		line:        make([]byte, lineLength),
		used:        0,
		out:         out,
		haveWritten: false,
	}
}

func (l *lineBreaker) Write(b []byte) (n int, err error) {
	n = len(b)
	if n == 0 {
		return
	}
	if l.used == 0 && l.haveWritten {
		_, err = l.out.Write([]byte{'\n'})
		if err != nil {
			return
		}
	}
	if l.used+len(b) < l.lineLength {
		l.used += copy(l.line[l.used:], b)
		return
	}
	l.haveWritten = true
	_, err = l.out.Write(l.line[0:l.used])
	if err != nil {
		return
	}
	excess := l.lineLength - l.used
	l.used = 0
	_, err = l.out.Write(b[0:excess])
	if err != nil {
		return
	}
	_, err = l.Write(b[excess:])
	return
}

func (l *lineBreaker) Close() (err error) {
	if l.used > 0 {
		_, err = l.out.Write(l.line[0:l.used])
		if err != nil {
			return
		}
	}
	return
}

func (n *Mixfit) encodeMixMessage(plain []byte, chain []string) ([]byte, string, error) {
	if n.pubring == nil {
		return nil, "", fmt.Errorf("pubring not initialized - please download stats first")
	}
	if len(plain) > bodyBytes {
		return nil, "", fmt.Errorf("message too large: %d bytes (max %d)", len(plain), bodyBytes)
	}
	if len(chain) == 0 {
		return nil, "", fmt.Errorf("empty chain")
	}
	entryRemailer := chain[0]
	chainCopy := make([]string, len(chain))
	copy(chainCopy, chain)

	yamnMsg := n.encodeMsg(plain, chainCopy)
	return yamnMsg, entryRemailer, nil
}

func (n *Mixfit) encodeMsg(plain []byte, chain []string) []byte {
	var err error
	var hop string
	chainCopy := make([]string, len(chain))
	copy(chainCopy, chain)
	final := newSlotFinal()
	final.setNumChunks(1)
	final.setBodyBytes(len(plain))
	m := newEncMessage()
	m.setChainLength(len(chainCopy))
	length := m.setPlainText(plain)
	final.setBodyBytes(length)

	hop = popstr(&chainCopy)
	slotData := newSlotData()
	slotData.setExit()
	exitKey := crandom.Randbytes(32)
	slotData.setAesKey(exitKey)
	slotData.setPacketID(final.getPacketID())
	slotData.setPacketInfo(final.encode())

	exitRemailer, err := n.pubring.Get(hop)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: Remailer unknown in public keyring\n", hop)
		os.Exit(1)
	}

	nextHopAddr := exitRemailer.Address

	m.encryptBody(slotData.aesKey, final.aesIV)
	m.shiftHeaders()
	if len(chainCopy) > 0 {
		m.deterministic(0)
	}
	slotData.setTagHash(m.getAntiTag())
	slotDataBytes := slotData.encode()
	header := newEncodeHeader()
	header.setRecipient(exitRemailer.Keyid, exitRemailer.PK)
	m.insertHeader(header.encode(slotDataBytes))

	interHops := m.getIntermediateHops()
	for interHop := 0; interHop < interHops; interHop++ {
		hop = popstr(&chainCopy)
		remailer, err := n.pubring.Get(hop)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: Remailer unknown in public keyring\n", hop)
			os.Exit(1)
		}

		inter := newSlotIntermediate()
		inter.setPartialIV(m.getPartialIV(interHop))
		inter.setNextHop(nextHopAddr)
		nextHopAddr = remailer.Address

		slotData = newSlotData()
		slotData.setAesKey(m.getKey(interHop))
		slotData.setPacketInfo(inter.encode())
		m.encryptAll(interHop)
		m.shiftHeaders()
		m.deterministic(interHop + 1)
		slotData.setTagHash(m.getAntiTag())
		slotDataBytes = slotData.encode()
		header = newEncodeHeader()
		header.setRecipient(remailer.Keyid, remailer.PK)
		m.insertHeader(header.encode(slotDataBytes))
	}

	if len(chainCopy) != 0 {
		fmt.Fprintln(os.Stderr, "Error: after encoding, chain was not empty")
		os.Exit(1)
	}
	return m.getPayload()
}

func (n *Mixfit) buildMixMessage(plainText string, attachment *Attachment, chain string) (string, string, error) {
	var messageBuf bytes.Buffer
	input := []byte(plainText)
	chainParts := strings.Split(chain, ",")
	if len(chainParts) == 0 {
		return "", "", fmt.Errorf("empty chain")
	}
	if len(chainParts) > maxChainLength {
		return "", "", fmt.Errorf("chain length %d exceeds maximum %d", len(chainParts), maxChainLength)
	}

	for i := range chainParts {
		chainParts[i] = strings.TrimSpace(chainParts[i])
		if chainParts[i] == "" {
			return "", "", fmt.Errorf("empty hop in chain (consecutive commas?)")
		}
		if chainParts[i] == "*" {
			return "", "", fmt.Errorf("random chains (*) are not supported")
		}
		remailer, err := n.pubring.Get(chainParts[i])
		if err != nil {
			return "", "", fmt.Errorf("remailer '%s' not found in pubring", chainParts[i])
		}
		if len(remailer.PK) != 32 {
			return "", "", fmt.Errorf("remailer '%s' has invalid public key", chainParts[i])
		}
		if len(remailer.Keyid) != 16 {
			return "", "", fmt.Errorf("remailer '%s' has invalid key ID", chainParts[i])
		}
	}

	if attachment != nil {
		boundary, err := n.generateRandomBoundary()
		if err != nil {
			return "", "", fmt.Errorf("failed to generate boundary: %v", err)
		}
		var msg strings.Builder
		msg.WriteString("To: " + strings.TrimSpace(n.toEntry.Text) + "\n")
		if subject := strings.TrimSpace(n.subjectEntry.Text); subject != "" {
			encodedSubject := encodeMIMESubject(subject)
			msg.WriteString("Subject: " + encodedSubject + "\n")
		}
		if references := strings.TrimSpace(n.referencesEntry.Text); references != "" {
			msg.WriteString("References: " + references + "\n")
		}
		if followupTo := strings.TrimSpace(n.followupToEntry.Text); followupTo != "" {
			msg.WriteString("Followup-To: " + followupTo + "\n")
		}
		if newsgroups := strings.TrimSpace(n.newsgroupsEntry.Text); newsgroups != "" {
			msg.WriteString("Newsgroups: " + newsgroups + "\n")
		}
		msg.WriteString("MIME-Version: 1.0\n")
		msg.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\n")
		msg.WriteString("Content-Transfer-Encoding: 7bit\n")
		msg.WriteString("\n")
		msg.WriteString("This is a multi-part message in MIME format.\n")
		msg.WriteString("\n")
		msg.WriteString("--" + boundary + "\n")
		msg.WriteString(n.createMIMETextPart(plainText))
		if attachment != nil {
			msg.WriteString("--" + boundary + "\n")
			attachmentPart, err := n.createMIMEAttachmentPart(attachment)
			if err != nil {
				return "", "", err
			}
			msg.WriteString(attachmentPart)
		}
		msg.WriteString("--" + boundary + "--\n")
		input = []byte(msg.String())
	} else {
		var msg strings.Builder
		msg.WriteString("To: " + strings.TrimSpace(n.toEntry.Text) + "\n")
		if subject := strings.TrimSpace(n.subjectEntry.Text); subject != "" {
			encodedSubject := encodeMIMESubject(subject)
			msg.WriteString("Subject: " + encodedSubject + "\n")
		}
		if references := strings.TrimSpace(n.referencesEntry.Text); references != "" {
			msg.WriteString("References: " + references + "\n")
		}
		if followupTo := strings.TrimSpace(n.followupToEntry.Text); followupTo != "" {
			msg.WriteString("Followup-To: " + followupTo + "\n")
		}
		if newsgroups := strings.TrimSpace(n.newsgroupsEntry.Text); newsgroups != "" {
			msg.WriteString("Newsgroups: " + newsgroups + "\n")
		}

		msg.WriteString("MIME-Version: 1.0\n")
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\n")

		msg.WriteString("\n")
		msg.WriteString(plainText)
		input = []byte(msg.String())
	}

	payload, entryRemailer, err := n.encodeMixMessage(input, chainParts)
	if err != nil {
		return "", "", err
	}

	remailer, err := n.pubring.Get(entryRemailer)
	if err != nil {
		return "", "", fmt.Errorf("remailer '%s' not found: %v", entryRemailer, err)
	}

	messageBuf.WriteString(fmt.Sprintf("To: %s\n", remailer.Address))
	messageBuf.WriteString("From: mix@nowhere.invalid\n")
	messageBuf.WriteString(fmt.Sprintf("Subject: yamn-%s\n\n", version))
	messageBuf.WriteString("::\n")
	messageBuf.WriteString(fmt.Sprintf("Remailer-Type: yamn-%s\n\n", version))
	messageBuf.WriteString("-----BEGIN REMAILER MESSAGE-----\n")
	messageBuf.WriteString(fmt.Sprintf("%d\n", len(payload)))
	digest, err := blake2s.New256(nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create digest: %v", err)
	}
	digest.Write(payload)
	messageBuf.WriteString(hex.EncodeToString(digest.Sum(nil)) + "\n")
	armorBuf := &bytes.Buffer{}
	wrap64(armorBuf, payload, base64LineWrap)
	messageBuf.Write(armorBuf.Bytes())
	messageBuf.WriteString("\n-----END REMAILER MESSAGE-----\n")

	return messageBuf.String(), entryRemailer, nil
}
