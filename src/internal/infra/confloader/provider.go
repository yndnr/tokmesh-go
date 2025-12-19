// Package confloader provides configuration loading mechanism.
package confloader

import "errors"

// ErrReadBytesNotSupported is returned when ReadBytes is called on a map provider.
var ErrReadBytesNotSupported = errors.New("confloader: ReadBytes not supported by map provider, use Read() instead")

// mapProvider is a simple koanf provider that loads configuration from a map.
//
// Note: koanf.Provider supports either ReadBytes() or Read() depending on the
// provider implementation; koanf will use whichever is available.
// For map-based providers, Read() is the appropriate method.
type mapProvider map[string]any

// ReadBytes returns an error as map provider doesn't support byte serialization.
// Use Read() instead.
func (m mapProvider) ReadBytes() ([]byte, error) {
	return nil, ErrReadBytesNotSupported
}

// Read returns the configuration map.
func (m mapProvider) Read() (map[string]any, error) {
	return m, nil
}

