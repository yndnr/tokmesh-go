package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveCLIConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.yaml")

	cfg, err := loadCLIConfig(path)
	if err != nil {
		t.Fatalf("load empty config: %v", err)
	}
	cfg.Profiles["ops"] = cliProfile{Cert: "/cert.pem", Key: "/key.pem", CA: "/ca.pem"}
	if err := saveCLIConfig(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := loadCLIConfig(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if _, ok := loaded.Profiles["ops"]; !ok {
		t.Fatalf("expected profile ops")
	}
}

func TestResolveProfilePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.yaml")
	os.Setenv("TOKMESH_CLI_CONFIG", path)
	t.Cleanup(func() { os.Unsetenv("TOKMESH_CLI_CONFIG") })

	cfg := &cliConfig{Profiles: map[string]cliProfile{
		"ops": {Cert: "/cert.pem", Key: "/key.pem", CA: "/ca.pem", APIKey: "secret"},
	}}
	if err := saveCLIConfig(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cert, key, ca, apiKey, err := resolveProfilePaths("ops", "", "", "", "")
	if err != nil {
		t.Fatalf("resolve profile: %v", err)
	}
	if cert != "/cert.pem" || key != "/key.pem" || ca != "/ca.pem" || apiKey != "secret" {
		t.Fatalf("unexpected values: %s %s %s %s", cert, key, ca, apiKey)
	}

	if _, _, _, _, err := resolveProfilePaths("missing", "", "", "", ""); err == nil {
		t.Fatalf("expected error for missing profile")
	}
}
