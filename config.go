package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func getConfigPath() string {
	return filepath.Join(getBaseDir(), configFileName)
}

func createDefaultUnifiedConfig(configPath string) error {
	defaultConfig := UnifiedConfig{
		LocalArticleDir: filepath.Join(getBaseDir(), "articles"),
		NNTPConfig: NNTPConfig{
			Server:      "news.tcpreset.net",
			Port:        119,
			Newsgroup:   "alt.anonymous.messages",
			LastArticle: 0,
		},
		PingerConfig: getDefaultPingerConfig(),
	}
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func loadOrCreateUnifiedConfig() (*UnifiedConfig, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createDefaultUnifiedConfig(configPath); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config UnifiedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config.LocalArticleDir == "" {
		config.LocalArticleDir = filepath.Join(getBaseDir(), "articles")
	}
	return &config, nil
}

func getDefaultPingerConfig() PingerConfig {
	return PingerConfig{
		Pingers: []Pinger{
			{
				Name: "Victor",
				URLs: struct {
					Pubring string `json:"pubring"`
					Mlist2  string `json:"mlist2"`
				}{
					Pubring: "https://echolot.virebent.art/pubring.mix",
					Mlist2:  "https://echolot.virebent.art/mlist2.txt",
				},
			},
			{
				Name: "Frell",
				URLs: struct {
					Pubring string `json:"pubring"`
					Mlist2  string `json:"mlist2"`
				}{
					Pubring: "http://echolot.theremailer.net/yamn/pubring.mix",
					Mlist2:  "http://echolot.theremailer.net/yamn/mlist2.txt",
				},
			},
			{
				Name: "Mixmin",
				URLs: struct {
					Pubring string `json:"pubring"`
					Mlist2  string `json:"mlist2"`
				}{
					Pubring: "https://www.mixmin.net/yamn/pubring.mix",
					Mlist2:  "https://www.mixmin.net/yamn/mlist2.txt",
				},
			},			
		},
		Proxy:          "127.0.0.1:1080",
		SMTPServer:     "mailrelay.archiade.net",
		SMTPPort:       587,
		SMTPUseTLS:     true,
		SMTPSkipVerify: true,
	}
}

func (n *Mixfit) saveUnifiedConfig() error {
	configPath := getConfigPath()
	data, err := json.MarshalIndent(n.unifiedConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func (n *Mixfit) initDB() error {
	dbPath := filepath.Join(getBaseDir(), "esub_rc.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	var err error
	n.db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	_, err = n.db.Exec(`CREATE TABLE IF NOT EXISTS esubs (esub_hex TEXT PRIMARY KEY, first_seen TEXT NOT NULL, article_id INTEGER, newsgroup TEXT)`)
	if err != nil {
		return err
	}
	rows, err := n.db.Query("SELECT esub_hex FROM esubs")
	if err != nil {
		return err
	}
	defer rows.Close()
	n.replayCache = make(map[string]bool)
	for rows.Next() {
		var esubHex string
		if scanErr := rows.Scan(&esubHex); scanErr != nil {
			return scanErr
		}
		n.replayCache[esubHex] = true
	}
	return nil
}
