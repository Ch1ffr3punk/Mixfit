package main

import (
	"database/sql"
	"errors"
	"io"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/proxy"
)

type FileInfo struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Chunks int    `json:"chunks"`
}

type FileChunk struct {
	Type   string `json:"type"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
	Data   []byte `json:"data"`
	IsLast bool   `json:"is_last"`
}

type UnifiedConfig struct {
	LocalArticleDir string       `json:"local_article_dir"`
	NNTPConfig      NNTPConfig   `json:"nntp_config"`
	PingerConfig    PingerConfig `json:"pinger_config"`
}

type NNTPConfig struct {
	Server      string `json:"server"`
	Port        int    `json:"port"`
	Newsgroup   string `json:"newsgroup"`
	LastArticle int    `json:"last_article"`
}

type PingerConfig struct {
	Pingers        []Pinger `json:"pingers"`
	Proxy          string   `json:"proxy"`
	SMTPServer     string   `json:"smtp_server"`
	SMTPPort       int      `json:"smtp_port"`
	SMTPUseTLS     bool     `json:"smtp_use_tls"`
	SMTPSkipVerify bool     `json:"smtp_skip_verify"`
}

type Pinger struct {
	Name string `json:"name"`
	URLs struct {
		Pubring string `json:"pubring"`
		Mlist2  string `json:"mlist2"`
	} `json:"urls"`
}

type Remailer struct {
	Name    string
	Address string
	Keyid   []byte
	PK      []byte
	Version string
	Type    string
	Expires time.Time
}

type Stats struct {
	Latent     int
	Uptime     float64
	Options    string
	LatentStr  string
	UptimeStr  string
	LatentHist string
	UptimeHist string
}

type Pubring struct {
	remailers map[string]*Remailer
	stats     map[string]*Stats
}

type Mixfit struct {
	app               fyne.App
	window            fyne.Window
	toEntry           *FocusAwareEntry
	subjectEntry      *FocusAwareEntry
	referencesEntry   *FocusAwareEntry
	followupToEntry   *FocusAwareEntry
	newsgroupsEntry   *FocusAwareEntry
	textArea          *FocusAwareMultiLineEntry
	chainEntry        *FocusAwareEntry
	isDarkTheme       bool
	statusLabel       *widget.Label
	statusDetail      *widget.Label
	targetURL         string
	targetDomain      string
	mainScroll        *container.Scroll
	headerSection     fyne.CanvasObject
	bottomSection     fyne.CanvasObject
	bottomVisible     bool
	hideTimer         *time.Timer
	themeSwitch       *widget.Button
	infoBtn           *widget.Button
	configBtn         *widget.Button
	unifiedConfig     *UnifiedConfig
	pool              *ConnectionPool
	db                *sql.DB
	replayCache       map[string]bool
	dbMutex           sync.RWMutex
	progressBar       *widget.ProgressBar
	progressLabel     *widget.Label
	progressContainer *fyne.Container
	statusResetTimer  *time.Timer
	currentAttachment *Attachment
	articlesDir       string
	pubring           *Pubring
	haveStats         bool
	mixChain          string
}

type slotFinal struct {
	aesIV          []byte
	chunkNum       uint8
	numChunks      uint8
	messageID      []byte
	packetID       []byte
	gotBodyBytes   bool
	bodyBytes      int
	deliveryMethod uint8
}

type slotData struct {
	version       uint8
	packetType    uint8
	protocol      uint8
	packetID      []byte
	gotAesKey     bool
	aesKey        []byte
	timestamp     []byte
	gotPacketInfo bool
	packetInfo    []byte
	gotTagHash    bool
	tagHash       []byte
}

type encodeHeader struct {
	gotRecipient   bool
	recipientKeyID []byte
	recipientPK    [32]byte
}

type encMessage struct {
	gotPayload       bool
	payload          []byte
	plainLength      int
	keys             [maxChainLength - 1][]byte
	ivs              [maxChainLength - 1][]byte
	chainLength      int
	intermediateHops int
	padHeaders       int
	padBytes         int
}

type slotIntermediate struct {
	gotAesIV12 bool
	aesIV12    []byte
	nextHop    []byte
}

type lineBreaker struct {
	lineLength  int
	line        []byte
	used        int
	out         io.Writer
	haveWritten bool
}

type ConnectionPool struct {
	dialer proxy.Dialer
	mu     sync.Mutex
}

type FocusAwareEntry struct {
	widget.Entry
	onFocusChanged func(bool)
}

type FocusAwareMultiLineEntry struct {
	widget.Entry
	onFocusChanged func(bool)
}

type esub struct {
	key     string
	subject string
}

type fileFilter struct {
	name     string
	patterns []string
}

type Attachment struct {
	Data        []byte
	Filename    string
	ContentType string
}

type oliveThemeWrapper struct {
	base fyne.Theme
}

type Scanner struct {
	data   []byte
	pos    int
	length int
}

type fixedWidthLayout struct {
	width float32
}

var pool = &ConnectionPool{}

var ErrNoNewArticles = errors.New("no new articles to fetch")
