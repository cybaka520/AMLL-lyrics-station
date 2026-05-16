package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Git struct {
		RepoURL   string `yaml:"repo_url"`
		LocalPath string `yaml:"local_path"`
	} `yaml:"git"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"database"`
	Sync struct {
		RateLimit int `yaml:"rate_limit"`
	} `yaml:"sync"`
	Log struct {
		Level      string `yaml:"level"`
		Path       string `yaml:"path"`
		MaxSize    int    `yaml:"max_size"`
		MaxBackups int    `yaml:"max_backups"`
		MaxAge     int    `yaml:"max_age"`
	} `yaml:"log"`
	API struct {
		Key string `yaml:"key"`
	} `yaml:"api"`
}

func Default() *Config {
	cfg := &Config{}
	cfg.Git.LocalPath = filepath.Join(appRoot(), "lyrics")
	cfg.Sync.RateLimit = 5
	cfg.Log.Level = "info"
	cfg.Log.Path = filepath.Join(appRoot(), "logs", "app.log")
	cfg.Log.MaxSize = 50
	cfg.Log.MaxBackups = 10
	cfg.Log.MaxAge = 30
	cfg.API.Key = "change-me"
	return cfg
}

func appRoot() string {
	if root := os.Getenv("APP_ROOT"); root != "" {
		return root
	}
	if wd, err := os.Getwd(); err == nil {
		if base := filepath.Base(wd); base != "go-build" && base != "tmp" {
			return wd
		}
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = filepath.Join("backend", "internal", "config", "config.yaml")
	}
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if os.Getenv("CONFIG_REQUIRED") == "1" {
				return nil, fmt.Errorf("config file not found: %s", path)
			}
		} else {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		if err := decryptPassword(cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Git.LocalPath == "" {
		cfg.Git.LocalPath = filepath.Join(appRoot(), "lyrics")
	}
	if cfg.API.Key == "" || cfg.API.Key == "change-me" {
		return nil, errors.New("api key is required")
	}
	return cfg, nil
}

func decryptPassword(cfg *Config) error {
	if cfg.Database.Password == "" {
		return nil
	}
	key := os.Getenv("DB_PASSWORD_KEY")
	if key == "" {
		return errors.New("DB_PASSWORD_KEY is required when database password is encrypted")
	}
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid DB_PASSWORD_KEY: %w", err)
	}
	if n := len(decodedKey); n != 16 && n != 24 && n != 32 {
		return fmt.Errorf("invalid DB_PASSWORD_KEY length: got %d bytes, want 16, 24, or 32", n)
	}
	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return fmt.Errorf("invalid DB_PASSWORD_KEY: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(cfg.Database.Password)
	if err != nil {
		return err
	}
	if len(decoded) < gcm.NonceSize() {
		return errors.New("invalid encrypted password")
	}
	nonce, ciphertext := decoded[:gcm.NonceSize()], decoded[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}
	cfg.Database.Password = string(plain)
	return nil
}
