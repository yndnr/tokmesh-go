// Package config defines the server configuration structure.
package config

import (
	"errors"
	"os"
)

// Verify validates the configuration.
func Verify(cfg *ServerConfig) error {
	if err := verifyServer(&cfg.Server); err != nil {
		return err
	}
	if err := verifyStorage(&cfg.Storage); err != nil {
		return err
	}
	return nil
}

func verifyServer(cfg *ServerSection) error {
	// TODO: Validate address formats
	// TODO: Check for port conflicts
	// TODO: Verify TLS cert/key files exist if specified
	return nil
}

func verifyStorage(cfg *StorageSection) error {
	if cfg.DataDir == "" {
		return errors.New("storage.data_dir is required")
	}

	// Check if data directory exists or can be created
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		return errors.New("cannot create data directory: " + err.Error())
	}

	if cfg.SnapshotKeep < 1 {
		return errors.New("storage.snapshot_keep must be at least 1")
	}

	return nil
}
