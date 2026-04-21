package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/data219/huectl/internal/domain"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       int             `yaml:"version" json:"version"`
	DefaultBridge string          `yaml:"default_bridge,omitempty" json:"default_bridge,omitempty"`
	Bridges       []domain.Bridge `yaml:"bridges" json:"bridges"`
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func DefaultPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(cfgDir, "huectl", "config.yaml"), nil
}

func (s *FileStore) Path() string {
	return s.path
}

func (s *FileStore) Load() (Config, *domain.AppError) {
	if s.path == "" {
		return Config{}, domain.NewError("CONFIG_PATH", "config path must not be empty", domain.ExitInternal)
	}

	stat, err := os.Stat(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{Version: 1, Bridges: []domain.Bridge{}}, nil
		}
		return Config{}, domain.WrapError("CONFIG_READ", "failed to read config file metadata", domain.ExitInternal, err)
	}

	mode := stat.Mode().Perm()
	if mode&0o077 != 0 {
		return Config{}, &domain.AppError{
			Code:     "CONFIG_PERMISSIONS",
			Message:  "config file must not be accessible by group or others",
			ExitCode: domain.ExitAuth,
			Details: map[string]any{
				"path": s.path,
				"mode": mode.String(),
			},
			Hints: []string{"Run chmod 600 on the config file"},
		}
	}

	payload, readErr := os.ReadFile(s.path)
	if readErr != nil {
		return Config{}, domain.WrapError("CONFIG_READ", "failed to read config file", domain.ExitInternal, readErr)
	}

	var cfg Config
	if unmarshalErr := yaml.Unmarshal(payload, &cfg); unmarshalErr != nil {
		return Config{}, domain.WrapError("CONFIG_PARSE", "failed to parse config yaml", domain.ExitUsage, unmarshalErr)
	}

	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Bridges == nil {
		cfg.Bridges = []domain.Bridge{}
	}
	return cfg, nil
}

func (s *FileStore) Save(cfg Config) *domain.AppError {
	if s.path == "" {
		return domain.NewError("CONFIG_PATH", "config path must not be empty", domain.ExitInternal)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Bridges == nil {
		cfg.Bridges = []domain.Bridge{}
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return domain.WrapError("CONFIG_DIR", "failed to create config directory", domain.ExitInternal, err)
	}

	payload, err := yaml.Marshal(cfg)
	if err != nil {
		return domain.WrapError("CONFIG_SERIALIZE", "failed to serialize config", domain.ExitInternal, err)
	}

	tmp := s.path + ".tmp"
	if writeErr := os.WriteFile(tmp, payload, 0o600); writeErr != nil {
		return domain.WrapError("CONFIG_WRITE", "failed to write temp config", domain.ExitInternal, writeErr)
	}
	if renameErr := os.Rename(tmp, s.path); renameErr != nil {
		_ = os.Remove(tmp)
		return domain.WrapError("CONFIG_WRITE", "failed to move temp config into place", domain.ExitInternal, renameErr)
	}
	if chmodErr := os.Chmod(s.path, 0o600); chmodErr != nil {
		return domain.WrapError("CONFIG_PERMISSIONS", "failed to set config permissions", domain.ExitInternal, chmodErr)
	}

	return nil
}
