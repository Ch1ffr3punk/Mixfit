package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NewScanner(data []byte) *Scanner {
	return &Scanner{
		data:   data,
		pos:    0,
		length: len(data),
	}
}

func (s *Scanner) Scan() bool {
	if s.pos >= s.length {
		return false
	}
	return true
}

func (s *Scanner) Text() string {
	if s.pos >= s.length {
		return ""
	}
	start := s.pos
	for s.pos < s.length && s.data[s.pos] != '\n' {
		s.pos++
	}
	end := s.pos
	if s.pos < s.length && s.data[s.pos] == '\n' {
		s.pos++
	}
	return string(s.data[start:end])
}

func (p *Pubring) ImportPubring(data []byte) error {
	scanner := NewScanner(data)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if !strings.Contains(fields[1], "@") {
			continue
		}
		remailer := &Remailer{
			Name:    fields[0],
			Address: fields[1],
			Keyid:   make([]byte, 16),
		}
		if len(fields[2]) == 32 {
			keyid, err := hex.DecodeString(fields[2])
			if err == nil {
				copy(remailer.Keyid, keyid)
			}
		}

		versionParts := strings.Split(fields[3], ":")
		if len(versionParts) >= 2 {
			remailer.Version = versionParts[1]
		}

		if len(fields) > 4 {
			remailer.Type = fields[4]
		}

		if len(fields) > 6 {
			if expires, err := time.Parse("2006-01-02", fields[6]); err == nil {
				remailer.Expires = expires
			}
		}

		for scanner.Scan() {
			keyLine := scanner.Text()
			if strings.Contains(keyLine, "-----Begin Mix Key-----") {
				continue
			}
			if strings.Contains(keyLine, "-----End Mix Key-----") {
				break
			}
			keyLine = strings.TrimSpace(keyLine)
			if len(keyLine) == 64 {
				pk, err := hex.DecodeString(keyLine)
				if err == nil && len(pk) == 32 {
					remailer.PK = pk
					p.remailers[remailer.Name] = remailer
				}
			}
		}
	}

	if len(p.remailers) == 0 {
		return fmt.Errorf("no remailers found in pubring")
	}

	return nil
}

func (p *Pubring) ImportStats(data []byte) error {
	lines := strings.Split(string(data), "\n")
	inStats := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "Mixmaster") && strings.Contains(line, "Latent-Hist") {
			inStats = true
			continue
		}
		if !inStats {
			continue
		}
		if strings.Contains(line, "Broken type-I remailer chains:") ||
			strings.Contains(line, "Broken type-II remailer chains:") ||
			strings.Contains(line, "Remailer-Capabilities:") {
			break
		}
		if strings.HasPrefix(line, "Stats-Version:") ||
			strings.HasPrefix(line, "Generated:") ||
			strings.HasPrefix(line, "-----") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			name := fields[0]

			if strings.ContainsAny(name, ".:/") {
				continue
			}

			latentHist := ""
			latentStr := ""
			uptimeHist := ""
			uptimeStr := ""
			options := ""

			if len(fields) >= 6 {
				latentHist = fields[1]
				latentStr = fields[2]
				uptimeHist = fields[3]
				uptimeStr = fields[4]
				options = fields[len(fields)-1]
			} else if len(fields) == 5 {
				latentHist = fields[1]
				latentStr = fields[2]
				uptimeHist = fields[3]
				uptimeStr = fields[4]
			}

			latent := 0
			if strings.Contains(latentStr, ":") {
				parts := strings.Split(latentStr, ":")
				if len(parts) == 2 {
					min, _ := strconv.Atoi(parts[0])
					sec, _ := strconv.Atoi(parts[1])
					latent = min*60 + sec
					if min < 10 {
						latentStr = fmt.Sprintf("0%d:%02d", min, sec)
					} else {
						latentStr = fmt.Sprintf("%d:%02d", min, sec)
					}
				}
			}

			uptime := 0.0
			if strings.HasSuffix(uptimeStr, "%") {
				uptime, _ = strconv.ParseFloat(strings.TrimSuffix(uptimeStr, "%"), 64)
			}

			p.stats[name] = &Stats{
				Latent:     latent,
				Uptime:     uptime,
				Options:    options,
				LatentStr:  latentStr,
				UptimeStr:  uptimeStr,
				LatentHist: latentHist,
				UptimeHist: uptimeHist,
			}
		}
	}

	return nil
}

func (p *Pubring) Get(name string) (*Remailer, error) {
	if remailer, ok := p.remailers[name]; ok {
		return remailer, nil
	}
	return nil, fmt.Errorf("remailer %s not found", name)
}

func (p *Pubring) GetStats(name string) *Stats {
	if stats, ok := p.stats[name]; ok {
		return stats
	}
	return nil
}

func getRemailerRole(options string) string {
	if strings.Contains(strings.ToLower(options), "d") {
		return "M"
	}
	return "E"
}
