package bastion

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
)

const defaultHostKeyPath = "bastion_host_key"

type Config struct {
	Addr               string
	HostKeyPath        string
	AuthorizedKeysPath string
	CAKeyPath          string
	GrantStore         *GrantStore
	Logger             *Logger
}

type Server struct {
	config         Config
	sshConfig      *ssh.ServerConfig
	listener       net.Listener
	closed         bool
	mu             sync.Mutex
	authorizedKeys map[string]struct{}
	logger         *Logger
	ca             *CertificateAuthority
	grants         *GrantStore
}

func NewServer(config Config) (*Server, error) {
	if config.Addr == "" {
		return nil, errors.New("bastion address required")
	}
	if config.HostKeyPath == "" {
		config.HostKeyPath = defaultHostKeyPath
	}

	signer, err := loadOrCreateHostKey(config.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load host key: %w", err)
	}

	authorizedKeys, err := loadAuthorizedKeys(config.AuthorizedKeysPath)
	if err != nil {
		return nil, fmt.Errorf("load authorized keys: %w", err)
	}
	if config.Logger == nil {
		return nil, errors.New("bastion logger required")
	}
	if config.GrantStore == nil {
		return nil, errors.New("bastion grant store required")
	}

	ca, err := NewCertificateAuthority(config.CAKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load CA: %w", err)
	}

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if cert, ok := key.(*ssh.Certificate); ok {
				if err := ca.ValidateUserCertificate(cert, conn.User()); err != nil {
					return nil, err
				}
				return &ssh.Permissions{Extensions: map[string]string{
					"principal": conn.User(),
				}}, nil
			}
			if len(authorizedKeys) == 0 {
				return nil, errors.New("no authorized keys configured")
			}
			if _, ok := authorizedKeys[string(key.Marshal())]; ok {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key for %s", conn.User())
		},
	}
	sshConfig.AddHostKey(signer)

	return &Server{
		config:         config,
		sshConfig:      sshConfig,
		authorizedKeys: authorizedKeys,
		logger:         config.Logger,
		ca:             ca,
		grants:         config.GrantStore,
	}, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = listener
	go s.serve()
	return nil
}

func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Addr
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.isClosed() {
				return
			}
			log.Printf("bastion accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Server) handleConn(netConn net.Conn) {
	defer netConn.Close()
	sshConn, channels, requests, err := ssh.NewServerConn(netConn, s.sshConfig)
	if err != nil {
		log.Printf("bastion ssh handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(requests)

	for channel := range channels {
		switch channel.ChannelType() {
		case "direct-tcpip":
			go s.handleDirectTCPIP(sshConn, channel)
		default:
			channel.Reject(ssh.UnknownChannelType, "unsupported channel type")
		}
	}
}

func loadOrCreateHostKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return ssh.ParsePrivateKey(data)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	pemBytes := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(pemBytes)
}

func loadAuthorizedKeys(path string) (map[string]struct{}, error) {
	authorized := make(map[string]struct{})
	if path == "" {
		return authorized, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	for len(data) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return nil, err
		}
		authorized[string(pubKey.Marshal())] = struct{}{}
		data = rest
	}
	return authorized, nil
}
