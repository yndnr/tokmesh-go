package config

import (
	"os"
)

type TLSConfig struct {
	EnableTLS       bool
	CertFile        string
	KeyFile         string
	CAFile          string
	RequireClientCA bool
}

type Config struct {
	BusinessListenAddr string
	AdminListenAddr    string

	TLSBusiness TLSConfig
	TLSAdmin    TLSConfig

	DataDir  string
	CertDir  string
	MemLimit uint64
}

func FromEnv() Config {
	return Config{
		BusinessListenAddr: getenv("TOKMESH_BUSINESS_ADDR", ":8080"),
		AdminListenAddr:    getenv("TOKMESH_ADMIN_ADDR", ":8081"),
		DataDir:            getenv("TOKMESH_DATA_DIR", "./data"),
		CertDir:            getenv("TOKMESH_CERT_DIR", "./certs"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

