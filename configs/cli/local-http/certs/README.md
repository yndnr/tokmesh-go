本 profile 使用 HTTP（明文），不需要 TLS/CA 文件。

安全提示：
- `http://127.0.0.1:*` 仍视为网络接口，必须提供 API Key（可通过 `-k/--api-key` 或交互输入）。
- 生产环境不建议在配置文件中明文保存 API Key。
