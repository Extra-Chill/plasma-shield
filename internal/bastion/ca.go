package bastion

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

const defaultCAKeyPath = "bastion_ca_key"

// CertificateAuthority manages SSH certificate signing and validation.
type CertificateAuthority struct {
	signer    ssh.Signer
	publicKey ssh.PublicKey
	keyPath   string
	now       func() time.Time
}

// NewCertificateAuthority loads or creates a CA keypair.
func NewCertificateAuthority(path string) (*CertificateAuthority, error) {
	return NewCertificateAuthorityWithClock(path, func() time.Time { return time.Now().UTC() })
}

// NewCertificateAuthorityWithClock loads or creates a CA keypair with a custom clock.
func NewCertificateAuthorityWithClock(path string, now func() time.Time) (*CertificateAuthority, error) {
	if now == nil {
		panic("bastion: nil clock")
	}
	if path == "" {
		path = defaultCAKeyPath
	}

	signer, publicKey, err := loadOrCreateCAKey(path)
	if err != nil {
		return nil, err
	}

	return &CertificateAuthority{
		signer:    signer,
		publicKey: publicKey,
		keyPath:   path,
		now:       now,
	}, nil
}

// PublicKey returns the CA public key.
func (c *CertificateAuthority) PublicKey() ssh.PublicKey {
	return c.publicKey
}

// IssueUserCertificate signs a short-lived user certificate tied to a grant.
func (c *CertificateAuthority) IssueUserCertificate(publicKey ssh.PublicKey, grant *Grant) (*ssh.Certificate, error) {
	if publicKey == nil {
		return nil, errors.New("public key required")
	}
	if grant == nil {
		return nil, errors.New("grant required")
	}

	now := c.now()
	if !grant.ExpiresAt.After(now) {
		return nil, errors.New("grant expired")
	}

	cert := &ssh.Certificate{
		Key:             publicKey,
		Serial:          uint64(now.UnixNano()),
		CertType:        ssh.UserCert,
		KeyId:           grant.ID,
		ValidPrincipals: []string{grant.Principal},
		ValidAfter:      uint64(now.Unix()),
		ValidBefore:     uint64(grant.ExpiresAt.Unix()),
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"grant_id": grant.ID,
				"target":   grant.Target,
			},
		},
	}

	if err := cert.SignCert(rand.Reader, c.signer); err != nil {
		return nil, fmt.Errorf("sign certificate: %w", err)
	}
	return cert, nil
}

// ValidateUserCertificate verifies a user certificate against the CA.
func (c *CertificateAuthority) ValidateUserCertificate(cert *ssh.Certificate, principal string) error {
	if cert == nil {
		return errors.New("certificate required")
	}

	checker := ssh.CertChecker{
		IsUserAuthority: func(key ssh.PublicKey) bool {
			return bytes.Equal(key.Marshal(), c.publicKey.Marshal())
		},
	}

	return checker.CheckCert(principal, cert)
}

func loadOrCreateCAKey(path string) (ssh.Signer, ssh.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, nil, err
		}
		return signer, signer.PublicKey(), nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, nil, err
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}
	pemBytes := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return nil, nil, err
	}

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, nil, err
	}

	pubBytes := ssh.MarshalAuthorizedKey(signer.PublicKey())
	_ = os.WriteFile(path+".pub", pubBytes, 0644)

	return signer, signer.PublicKey(), nil
}
