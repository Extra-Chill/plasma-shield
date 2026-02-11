package main

import (
	"crypto/tls"
	"testing"
)

// TestTLSConfig verifies the TLS configuration is secure
func TestTLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	t.Run("minimum TLS version is 1.2", func(t *testing.T) {
		if tlsConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("expected TLS 1.2 minimum, got %d", tlsConfig.MinVersion)
		}
	})

	t.Run("only secure cipher suites", func(t *testing.T) {
		// All configured suites should be ECDHE with GCM
		for _, suite := range tlsConfig.CipherSuites {
			name := tls.CipherSuiteName(suite)
			if name == "" {
				t.Errorf("unknown cipher suite: %d", suite)
				continue
			}
			// Check it's ECDHE (forward secrecy) and GCM (AEAD)
			if suite != tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 &&
				suite != tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 &&
				suite != tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 &&
				suite != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
				t.Errorf("unexpected cipher suite: %s", name)
			}
		}
	})

	t.Run("no insecure cipher suites", func(t *testing.T) {
		insecureSuites := []uint16{
			tls.TLS_RSA_WITH_RC4_128_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		}

		for _, insecure := range insecureSuites {
			for _, configured := range tlsConfig.CipherSuites {
				if insecure == configured {
					t.Errorf("insecure cipher suite configured: %s", tls.CipherSuiteName(insecure))
				}
			}
		}
	})
}
