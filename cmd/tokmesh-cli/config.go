package main

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type cliProfile struct {
	Cert   string `yaml:"cert"`
	Key    string `yaml:"key"`
	CA     string `yaml:"ca"`
	APIKey string `yaml:"api_key"`
}

type cliConfig struct {
	Profiles map[string]cliProfile `yaml:"profiles"`
}

// loadCLIConfig 从给定路径读取 CLI 配置文件。
// 如果文件不存在，返回一个包含已初始化 Profiles map 的空配置。
// 如果文件存在但无法读取或解析为 YAML，返回错误。
func loadCLIConfig(path string) (*cliConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &cliConfig{Profiles: make(map[string]cliProfile)}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]cliProfile)
	}
	return &cfg, nil
}

// saveCLIConfig 将 CLI 配置以 YAML 格式写入指定路径。
// 如果父目录不存在则创建（权限 0700）。
// 如果配置为 nil 或无法序列化，返回错误。
// 配置文件以 0600 权限写入以确保安全。
func saveCLIConfig(path string, cfg *cliConfig) error {
	if cfg == nil {
		return errors.New("nil cli config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// resolveProfilePaths 从命名 profile 解析证书路径。
// 如果 profile 为空，直接返回提供的 cert/key/ca/apiKey 路径。
// 命令行标志优先于 profile 中的值。
// 返回解析后的证书路径、密钥路径、CA 路径、API Key 及可能的错误。
func resolveProfilePaths(profile, cert, key, ca, apiKey string) (string, string, string, string, error) {
	if profile == "" {
		return cert, key, ca, apiKey, nil
	}
	cfgPath, err := cliConfigPath()
	if err != nil {
		return "", "", "", "", err
	}
	cfg, err := loadCLIConfig(cfgPath)
	if err != nil {
		return "", "", "", "", err
	}
	entry, ok := cfg.Profiles[profile]
	if !ok {
		return "", "", "", "", errors.New("profile not found")
	}
	if cert == "" {
		cert = entry.Cert
	}
	if key == "" {
		key = entry.Key
	}
	if ca == "" {
		ca = entry.CA
	}
	if apiKey == "" {
		apiKey = entry.APIKey
	}
	return cert, key, ca, apiKey, nil
}

// cliConfigPath 返回 CLI 配置文件路径。
// 首先检查 TOKMESH_CLI_CONFIG 环境变量，如未设置则回退到 ~/.tokmesh/cli.yaml。
// 如果无法确定用户主目录，返回错误。
func cliConfigPath() (string, error) {
	if v := os.Getenv("TOKMESH_CLI_CONFIG"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tokmesh", "cli.yaml"), nil
}
