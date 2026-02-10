package bastion

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestCertificateAuthorityLoadOrCreate(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "bastion_ca_key")

	ca, err := NewCertificateAuthority(keyPath)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat CA key: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected CA key mode 0600, got %o", info.Mode().Perm())
	}

	ca2, err := NewCertificateAuthority(keyPath)
	if err != nil {
		t.Fatalf("reload CA: %v", err)
	}
	if !bytes.Equal(ca.PublicKey().Marshal(), ca2.PublicKey().Marshal()) {
		t.Fatal("expected CA public key to be stable across reload")
	}
}

func TestIssueAndValidateUserCertificate(t *testing.T) {
	tempDir := t.TempDir()
	ca, err := NewCertificateAuthority(filepath.Join(tempDir, "bastion_ca_key"))
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	grant := &Grant{
		ID:        "grant-1",
		Principal: "alice",
		Target:    "agent-1",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	cert, err := ca.IssueUserCertificate(signer.PublicKey(), grant)
	if err != nil {
		t.Fatalf("issue cert: %v", err)
	}

	if err := ca.ValidateUserCertificate(cert, "alice"); err != nil {
		t.Fatalf("validate cert: %v", err)
	}

	if err := ca.ValidateUserCertificate(cert, "bob"); err == nil {
		t.Fatal("expected validation to fail for wrong principal")
	}
}

func TestIssueUserCertificateRejectsExpiredGrant(t *testing.T) {
	tempDir := t.TempDir()
	ca, err := NewCertificateAuthority(filepath.Join(tempDir, "bastion_ca_key"))
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	grant := &Grant{
		ID:        "grant-expired",
		Principal: "alice",
		Target:    "agent-1",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}

	if _, err := ca.IssueUserCertificate(signer.PublicKey(), grant); err == nil {
		t.Fatal("expected error issuing certificate for expired grant")
	}
}
