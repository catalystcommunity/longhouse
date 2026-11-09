package linkkeys

import (
	"errors"

	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
)

// NewPinnedTransport uses the same endpoint and fingerprint discovery as login.
// Application operations always require a pinned TLS connection.
func NewPinnedTransport(cfg TCPConfig) (*rp.PinnedRpcTransport, error) {
	if cfg.Insecure {
		return nil, errors.New("application operations require pinned TLS")
	}
	c, err := NewTCPClient(cfg)
	if err != nil {
		return nil, err
	}
	return rp.NewPinnedRpcTransport(rp.PinnedRpcTransportOptions{
		TCPAddress: c.addr, Fingerprints: c.fingerprints, APIKey: c.apiKey,
		RequestTimeout: c.dialTimeout + c.ioTimeout,
	})
}
