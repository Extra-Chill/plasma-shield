package bastion

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type directTCPIP struct {
	DestAddr string
	DestPort uint32
	OrigAddr string
	OrigPort uint32
}

func (s *Server) handleDirectTCPIP(sshConn *ssh.ServerConn, channel ssh.NewChannel) {
	var payload directTCPIP
	if err := ssh.Unmarshal(channel.ExtraData(), &payload); err != nil {
		channel.Reject(ssh.Prohibited, "invalid direct-tcpip payload")
		return
	}

	// Check grant BEFORE accepting channel
	grant := s.grants.ValidateAccess(sshConn.User(), payload.DestAddr)
	if grant == nil {
		channel.Reject(ssh.Prohibited, "no valid grant for target")
		return
	}

	conn, reqs, err := channel.Accept()
	if err != nil {
		log.Printf("bastion channel accept failed: %v", err)
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	address := net.JoinHostPort(payload.DestAddr, strconv.Itoa(int(payload.DestPort)))
	targetConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("bastion dial %s failed: %v", address, err)
		return
	}
	defer targetConn.Close()

	sessionID := fmt.Sprintf("%x-%d", sshConn.SessionID(), time.Now().UnixNano())
	principal := sshConn.User()
	grantID := grant.ID

	s.logger.LogConnect(sessionID, grantID, principal, address)
	defer s.logger.LogDisconnect(sessionID, grantID, principal, address)

	proxyBidirectional(conn, targetConn)
}

func proxyBidirectional(left io.ReadWriteCloser, right io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(left, right)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(right, left)
	}()

	wg.Wait()
}
