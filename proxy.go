package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/proxy"
)

func getBaseDir() string {
	if runtime.GOOS == "android" {
		return "/sdcard/Download/Mixfit"
	}
	return "."
}

func ensureBaseDir() error {
	baseDir := getBaseDir()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory %s: %v", baseDir, err)
	}
	articlesDir := filepath.Join(baseDir, "articles")
	if err := os.MkdirAll(articlesDir, 0755); err != nil {
		return fmt.Errorf("failed to create articles directory %s: %v", articlesDir, err)
	}
	return nil
}

func init() {
	if err := initProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Proxy initialization failed: %v\n", err)
	}
}

func initProxy() error {
	p, err := proxy.SOCKS5("tcp", proxyAddress, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to initialize SOCKS5 proxy: %v", err)
	}
	pool.dialer = p
	return nil
}

func (p *ConnectionPool) getConnection(target string) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dialer == nil {
		if err := initProxy(); err != nil {
			return nil, err
		}
	}
	for i := 0; i < maxRetries; i++ {
		conn, err := p.dialer.Dial("tcp", target)
		if err == nil {
			conn.SetDeadline(time.Now().Add(connectionTimeout))
			return conn, nil
		}
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}
	return nil, fmt.Errorf("connecting to %s via proxy after %d attempts", target, maxRetries)
}
