package bastion

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestBastionDirectTCPIPProxy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen target: %v", err)
	}
	defer listener.Close()

	targetAddr := listener.Addr().String()
	serverReady := make(chan struct{})

	go func() {
		close(serverReady)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		if string(buf) != "ping" {
			return
		}
		conn.Write([]byte("pong"))
	}()

	<-serverReady

	tempDir := t.TempDir()
	logStore := NewLogStore(10)
	logger := NewLogger(logStore)
	server, err := NewServer(Config{
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(tempDir, "bastion_host_key"),
		Logger:      logger,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	client, err := ssh.Dial("tcp", server.Addr(), &ssh.ClientConfig{
		User:            "tester",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ssh dial: %v", err)
	}
	defer client.Close()

	conn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		t.Fatalf("dial target through bastion: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if string(buf) != "pong" {
		t.Fatalf("unexpected response: %q", string(buf))
	}
}
