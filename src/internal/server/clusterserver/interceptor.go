// Package clusterserver provides the cluster communication server.
package clusterserver

import "context"

// Interceptor provides middleware for cluster RPC calls.
type Interceptor struct{}

// NewInterceptor creates a new Interceptor.
func NewInterceptor() *Interceptor {
	return &Interceptor{}
}

// ValidateMTLS validates the client certificate from mTLS handshake.
func (i *Interceptor) ValidateMTLS(ctx context.Context) error {
	// TODO: Extract peer certificate from context
	// TODO: Verify certificate against cluster CA
	// TODO: Extract node ID from certificate
	return nil
}

// LogRequest logs RPC request details.
func (i *Interceptor) LogRequest(ctx context.Context, method string) {
	// TODO: Log request method, peer node ID, timestamp
}

// LogResponse logs RPC response details.
func (i *Interceptor) LogResponse(ctx context.Context, method string, err error) {
	// TODO: Log response status, duration, error if any
}
