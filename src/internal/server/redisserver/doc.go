// Package redisserver provides Redis protocol compatible server for TokMesh.
//
// This package implements the RESP2 subset required by `RQ-0303`,
// using only the Go standard library (no third-party RESP server).
//
// Supported commands (see `specs/1-requirements/RQ-0303-业务接口规约-Redis协议.md`):
//   - PING, QUIT
//   - AUTH
//   - GET, SET, DEL, EXPIRE, TTL, EXISTS, SCAN
//   - TM.CREATE, TM.VALIDATE, TM.REVOKE_USER
//
// @req RQ-0303
// @design DS-0301
package redisserver
