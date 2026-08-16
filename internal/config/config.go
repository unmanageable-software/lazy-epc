package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultConfigName = "config.toml"

// Config contains the optional runtime settings shared by the CLI and TUI.
type Config struct {
	Database        string
	OutputDir       string
	TimestampOutput bool
}

func Default() Config {
	return Config{
		Database:        "payments.db",
		OutputDir:       ".",
		TimestampOutput: false,
	}
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "lazy-epc", DefaultConfigName)
	}
	return filepath.Join(home, ".config", "lazy-epc", DefaultConfigName)
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg.resolve(), nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")
		switch key {
		case "database":
			cfg.Database = expandPath(value)
		case "output_dir":
			cfg.OutputDir = expandPath(value)
		case "timestamp_output":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("parse %s: invalid boolean %q", key, value)
			}
			cfg.TimestampOutput = parsed
		}
	}
	return cfg.resolve(), nil
}

func LoadDefault() (Config, error) {
	return Load(DefaultPath())
}

func ExpandPath(value string) string {
	return expandPath(value)
}

func (c Config) resolve() Config {
	if c.OutputDir == "" {
		c.OutputDir = "."
	}
	c.OutputDir = expandPath(c.OutputDir)
	if c.Database == "" {
		c.Database = filepath.Join(c.OutputDir, "payments.db")
	}
	c.Database = expandPath(c.Database)
	return c
}

func (c Config) PaymentOutputPath(generatedAt time.Time) string {
	name := "payment.html"
	if c.TimestampOutput {
		name = fmt.Sprintf("payment-%s.html", generatedAt.Format("20060102-150405"))
	}
	return filepath.Join(c.OutputDir, name)
}

func expandPath(value string) string {
	if value == "" {
		return value
	}
	if value == "~" {
		homeDir := mustHomeDir()
		if homeDir == "" {
			return value
		}
		return homeDir
	}
	if strings.HasPrefix(value, "~/") {
		homeDir := mustHomeDir()
		if homeDir == "" {
			return value
		}
		return filepath.Join(homeDir, strings.TrimPrefix(value, "~/"))
	}
	if strings.HasPrefix(value, "~\\") {
		homeDir := mustHomeDir()
		if homeDir == "" {
			return value
		}
		return filepath.Join(homeDir, strings.TrimPrefix(value, "~\\"))
	}
	return value
}

func mustHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return home
}
