package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/yndnr/tokmesh-go/internal/security"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tokmesh-cli: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printGlobalHelp()
		return nil
	}
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "status":
		opts, err := parseServerFlags("status", rest, nil)
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doStatus(client, opts.admin, opts.apiKey)
	case "cleanup":
		opts, err := parseServerFlags("cleanup", rest, nil)
		if err != nil {
			return err
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doCleanup(client, opts.admin, opts.apiKey)
	case "revoke":
		opts, err := parseServerFlags("revoke", rest, func(fs *flag.FlagSet, o *serverCommandFlags) {
			fs.StringVar(&o.session, "session", o.session, "session ID")
			fs.StringVar(&o.session, "s", o.session, "session ID (shorthand)")
		})
		if err != nil {
			return err
		}
		if opts.session == "" {
			return errors.New("session flag required for revoke")
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doRevoke(client, opts.admin, opts.apiKey, opts.session)
	case "kick-user":
		opts, err := parseServerFlags("kick-user", rest, func(fs *flag.FlagSet, o *serverCommandFlags) {
			fs.StringVar(&o.user, "user", o.user, "user ID")
			fs.StringVar(&o.user, "u", o.user, "user ID (shorthand)")
			fs.StringVar(&o.device, "device", o.device, "device ID")
			fs.StringVar(&o.device, "d", o.device, "device ID (shorthand)")
		})
		if err != nil {
			return err
		}
		if opts.user == "" {
			return errors.New("user flag required for kick-user")
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doKickUser(client, opts.admin, opts.apiKey, opts.user, opts.device)
	case "kick-device":
		opts, err := parseServerFlags("kick-device", rest, func(fs *flag.FlagSet, o *serverCommandFlags) {
			fs.StringVar(&o.device, "device", o.device, "device ID")
			fs.StringVar(&o.device, "d", o.device, "device ID (shorthand)")
		})
		if err != nil {
			return err
		}
		if opts.device == "" {
			return errors.New("device flag required for kick-device")
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doKickDevice(client, opts.admin, opts.apiKey, opts.device)
	case "kick-tenant":
		opts, err := parseServerFlags("kick-tenant", rest, func(fs *flag.FlagSet, o *serverCommandFlags) {
			fs.StringVar(&o.tenant, "tenant", o.tenant, "tenant ID")
			fs.StringVar(&o.tenant, "t", o.tenant, "tenant ID (shorthand)")
		})
		if err != nil {
			return err
		}
		if opts.tenant == "" {
			return errors.New("tenant flag required for kick-tenant")
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doKickTenant(client, opts.admin, opts.apiKey, opts.tenant)
	case "list-sessions":
		opts, err := parseServerFlags("list-sessions", rest, func(fs *flag.FlagSet, o *serverCommandFlags) {
			fs.StringVar(&o.user, "user", o.user, "user ID filter")
			fs.StringVar(&o.user, "u", o.user, "user ID filter (shorthand)")
			fs.StringVar(&o.device, "device", o.device, "device ID filter")
			fs.StringVar(&o.device, "d", o.device, "device ID filter (shorthand)")
			fs.StringVar(&o.tenant, "tenant", o.tenant, "tenant ID filter")
			fs.StringVar(&o.tenant, "t", o.tenant, "tenant ID filter (shorthand)")
		})
		if err != nil {
			return err
		}
		client, err := opts.buildClient()
		if err != nil {
			return err
		}
		return doListSessions(client, opts.admin, opts.apiKey, opts.user, opts.device, opts.tenant)
	case "cert":
		return runCertCommand(rest)
	case "help":
		printGlobalHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func doStatus(client *http.Client, admin, apiKey string) error {
	req, err := http.NewRequest(http.MethodGet, admin+"/admin/status", nil)
	if err != nil {
		return err
	}
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status request failed: %s", data)
	}
	var result struct {
		Status   string `json:"status"`
		Sessions struct {
			Total   int `json:"total"`
			Active  int `json:"active"`
			Revoked int `json:"revoked"`
			Expired int `json:"expired"`
		} `json:"sessions"`
		Memory struct {
			Current uint64 `json:"current_bytes"`
			Limit   uint64 `json:"limit_bytes"`
		} `json:"memory"`
		Cleanup struct {
			IntervalSeconds int       `json:"interval_seconds"`
			LastRun         time.Time `json:"last_run"`
			LastRemoved     int       `json:"last_removed"`
		} `json:"cleanup"`
		Audit struct {
			Total    uint64 `json:"total"`
			Rejected uint64 `json:"rejected"`
		} `json:"audit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Admin status for %s\n", admin)
	fmt.Printf("  Sessions: total=%d active=%d revoked=%d expired=%d\n", result.Sessions.Total, result.Sessions.Active, result.Sessions.Revoked, result.Sessions.Expired)
	fmt.Printf("  Memory: current=%d limit=%d bytes\n", result.Memory.Current, result.Memory.Limit)
	fmt.Printf("  Cleanup: interval=%ds last_run=%s removed=%d\n", result.Cleanup.IntervalSeconds, result.Cleanup.LastRun.Format(time.RFC3339), result.Cleanup.LastRemoved)
	fmt.Printf("  Audit: total=%d rejected=%d\n", result.Audit.Total, result.Audit.Rejected)
	if data, err := json.MarshalIndent(result, "", "  "); err == nil {
		fmt.Printf("  Raw response:%s\n", string(data))
	}
	return nil
}

func doCleanup(client *http.Client, admin, apiKey string) error {
	reqBody, _ := json.Marshal(map[string]string{})
	req, err := http.NewRequest(http.MethodPost, admin+"/admin/session/cleanup", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cleanup failed: %s", data)
	}
	var data struct {
		Status   string    `json:"status"`
		Removed  int       `json:"removed"`
		RanAt    time.Time `json:"ran_at"`
		Interval float64   `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Cleanup: removed=%d ran_at=%s interval=%.0fs\n", data.Removed, data.RanAt.Format(time.RFC3339), data.Interval)
	return nil
}

func doRevoke(client *http.Client, admin, apiKey, sessionID string) error {
	payload, _ := json.Marshal(map[string]string{"id": sessionID})
	req, err := http.NewRequest(http.MethodPost, admin+"/admin/session/revoke", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke failed: %s", data)
	}
	var sessionResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Session %v revoked (status=%v)\n", sessionResp["id"], sessionResp["status"])
	return nil
}

func doKickUser(client *http.Client, admin, apiKey, userID, deviceID string) error {
	payload, _ := json.Marshal(map[string]string{
		"user_id":   userID,
		"device_id": deviceID,
	})
	req, err := http.NewRequest(http.MethodPost, admin+"/admin/session/kick/user", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick-user failed: %s", data)
	}
	var data struct {
		Status  string `json:"status"`
		Removed int    `json:"removed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Kick user %s removed %d sessions\n", userID, data.Removed)
	return nil
}

func doKickDevice(client *http.Client, admin, apiKey, deviceID string) error {
	payload, _ := json.Marshal(map[string]string{"device_id": deviceID})
	req, err := http.NewRequest(http.MethodPost, admin+"/admin/session/kick/device", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick-device failed: %s", data)
	}
	var data struct {
		Removed int `json:"removed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Kick device %s removed %d sessions\n", deviceID, data.Removed)
	return nil
}

func doKickTenant(client *http.Client, admin, apiKey, tenantID string) error {
	payload, _ := json.Marshal(map[string]string{"tenant_id": tenantID})
	req, err := http.NewRequest(http.MethodPost, admin+"/admin/session/kick/tenant", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick-tenant failed: %s", data)
	}
	var data struct {
		Removed int `json:"removed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] Kick tenant %s removed %d sessions\n", tenantID, data.Removed)
	return nil
}

func doListSessions(client *http.Client, admin, apiKey, userID, deviceID, tenantID string) error {
	req, err := http.NewRequest(http.MethodGet, admin+"/admin/session/list", nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	if userID != "" {
		q.Set("user_id", userID)
	}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	if tenantID != "" {
		q.Set("tenant_id", tenantID)
	}
	req.URL.RawQuery = q.Encode()
	setAPIKeyHeader(req, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list-sessions failed: %s", data)
	}
	var data struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Printf("[tokmesh-cli] List sessions returned %d result(s)\n", len(data.Sessions))
	for _, sess := range data.Sessions {
		fmt.Printf("  - id=%v user=%v tenant=%v device=%v status=%v expires_at=%v\n",
			sess["id"], sess["user_id"], sess["tenant_id"], sess["device_id"], sess["status"], sess["expires_at"])
	}
	return nil
}

func setAPIKeyHeader(req *http.Request, key string) {
	if key != "" {
		req.Header.Set(apiKeyHeader, key)
	}
}

const apiKeyHeader = "X-API-Key"

type serverCommandFlags struct {
	admin   string
	profile string
	cert    string
	key     string
	ca      string
	apiKey  string
	session string
	user    string
	device  string
	tenant  string
}

func parseServerFlags(name string, args []string, configure func(*flag.FlagSet, *serverCommandFlags)) (*serverCommandFlags, error) {
	fs, opts := newServerFlagSet(name)
	if configure != nil {
		configure(fs, opts)
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return opts, nil
}

func newServerFlagSet(name string) (*flag.FlagSet, *serverCommandFlags) {
	opts := &serverCommandFlags{
		admin:   envOrDefault("TOKMESH_ADMIN_ADDR", "http://127.0.0.1:8081"),
		profile: envOrDefault("TOKMESH_CLI_PROFILE", ""),
		cert:    envOrDefault("TOKMESH_CLI_CERT", ""),
		key:     envOrDefault("TOKMESH_CLI_KEY", ""),
		ca:      envOrDefault("TOKMESH_CLI_CA", ""),
		apiKey:  envOrDefault("TOKMESH_API_KEY", ""),
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&opts.admin, "admin", opts.admin, "admin API base URL")
	fs.StringVar(&opts.admin, "a", opts.admin, "shorthand for --admin")
	fs.StringVar(&opts.profile, "profile", opts.profile, "CLI certificate profile name")
	fs.StringVar(&opts.profile, "p", opts.profile, "shorthand for --profile")
	fs.StringVar(&opts.cert, "cert", opts.cert, "client TLS certificate path")
	fs.StringVar(&opts.cert, "c", opts.cert, "shorthand for --cert")
	fs.StringVar(&opts.key, "key", opts.key, "client TLS key path")
	fs.StringVar(&opts.key, "k", opts.key, "shorthand for --key")
	fs.StringVar(&opts.ca, "ca", opts.ca, "CA bundle path")
	fs.StringVar(&opts.apiKey, "api-key", opts.apiKey, "business API key")
	return fs, opts
}

func (o *serverCommandFlags) buildClient() (*http.Client, error) {
	certPath, keyPath, caPath, err := resolveProfilePaths(o.profile, o.cert, o.key, o.ca)
	if err != nil {
		return nil, err
	}
	return buildHTTPClient(certPath, keyPath, caPath)
}

func runCertCommand(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		printCertHelp()
		return nil
	}
	switch args[0] {
	case "csr":
		return runCertCSR(args[1:])
	case "install":
		return runCertInstall(args[1:])
	case "list":
		return runCertList()
	case "remove":
		return runCertRemove(args[1:])
	default:
		return fmt.Errorf("unknown cert subcommand %q", args[0])
	}
}

func runCertCSR(args []string) error {
	fs := flag.NewFlagSet("cert csr", flag.ContinueOnError)
	cn := fs.String("cn", "", "certificate common name")
	hosts := fs.String("hosts", "", "comma-separated DNS or IP SAN entries")
	csrOut := fs.String("out", "tokmesh-cli.csr", "CSR output path")
	keyOut := fs.String("key-out", "tokmesh-cli.key", "private key output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cn == "" {
		return errors.New("cn is required")
	}
	dnsNames, ipAddrs := security.ParseHosts(*hosts)
	result, err := security.GenerateCSR(security.CSRRequest{
		CommonName: *cn,
		DNSNames:   dnsNames,
		IPAddrs:    ipAddrs,
	})
	if err != nil {
		return err
	}
	if err := security.WritePrivateKeyToFile(*keyOut, result.PrivateKey); err != nil {
		return err
	}
	return security.WritePEMToFile(*csrOut, "CERTIFICATE REQUEST", result.CSR)
}

func runCertInstall(args []string) error {
	fs := flag.NewFlagSet("cert install", flag.ContinueOnError)
	profile := fs.String("profile", "", "profile name")
	cert := fs.String("cert", "", "certificate PEM path")
	key := fs.String("key", "", "private key PEM path")
	ca := fs.String("ca", "", "CA bundle PEM path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profile == "" || *cert == "" || *key == "" {
		return errors.New("profile, cert, and key are required")
	}
	cfgPath, err := cliConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadCLIConfig(cfgPath)
	if err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]cliProfile)
	}
	cfg.Profiles[*profile] = cliProfile{Cert: *cert, Key: *key, CA: *ca}
	return saveCLIConfig(cfgPath, cfg)
}

func runCertList() error {
	cfgPath, err := cliConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadCLIConfig(cfgPath)
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Println("no profiles configured")
		return nil
	}
	for name, profile := range cfg.Profiles {
		fmt.Printf("%s:\n  cert: %s\n  key: %s\n  ca: %s\n", name, profile.Cert, profile.Key, profile.CA)
	}
	return nil
}

func runCertRemove(args []string) error {
	fs := flag.NewFlagSet("cert remove", flag.ContinueOnError)
	profile := fs.String("profile", "", "profile name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profile == "" {
		return errors.New("profile is required")
	}
	cfgPath, err := cliConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadCLIConfig(cfgPath)
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[*profile]; !ok {
		return errors.New("profile not found")
	}
	delete(cfg.Profiles, *profile)
	return saveCLIConfig(cfgPath, cfg)
}

func buildHTTPClient(certFile, keyFile, caFile string) (*http.Client, error) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig, err := security.LoadTLSConfig(certFile, keyFile, caFile)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		base.TLSClientConfig = tlsConfig
	}
	return &http.Client{Transport: base}, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func printCertHelp() {
	fmt.Print(`tokmesh-cli cert help

可用子命令：
  csr       生成客户端证书 CSR 与私钥
            示例：tokmesh-cli -cmd cert csr -cn ops.example.com -hosts ops.example.com,127.0.0.1 -out csr.pem -key-out csr.key
  install   将证书/私钥/CA 安装到 CLI 配置文件
            示例：tokmesh-cli -cmd cert install -profile ops -cert ops.pem -key ops-key.pem -ca ca.pem
  list      查看已配置的证书 profile
            示例：tokmesh-cli -cmd cert list
  remove    删除指定 profile
            示例：tokmesh-cli -cmd cert remove -profile ops

配置文件默认位于 ~/.tokmesh/cli.yaml，可通过 TOKMESH_CLI_CONFIG 覆盖。`)
}

func printGlobalHelp() {
	fmt.Print(`tokmesh-cli command usage:

tokmesh-cli <command> [options]

可用命令：
  status            查看节点状态
  cleanup           触发 TTL 清理
  revoke            撤销指定 Session（需 --session/-s）
  kick-user         按 user/device 批量踢人（需 --user/-u）
  kick-device       按 device 批量踢人（需 --device/-d）
  kick-tenant       按 tenant 批量踢人（需 --tenant/-t）
  list-sessions     查询会话（可选 --user/--device/--tenant）
  cert              证书管理命令（csr/install/list/remove/help）

通用选项：
  --admin, -a      管理端地址，默认读取 TOKMESH_ADMIN_ADDR
  --profile, -p    CLI 证书 profile，默认 TOKMESH_CLI_PROFILE
  --cert/-c, --key/-k, --ca  直接指定证书/私钥/CA
`)
}
