本 profile 使用本地 Socket 通道，不需要 TLS/CA 文件。

平台说明：
- Linux/macOS：UDS，例如 `/var/run/tokmesh-server/tokmesh-server.sock`
- Windows：Named Pipe，例如 `\\\\.\\pipe\\tokmesh-server`

澄清：
- 本地 Socket/Named Pipe 由 `tokmesh-server` 暴露，`tokmesh-cli` 仅作为客户端连接；不使用 `tokmesh-cli.sock` 之类的路径。
