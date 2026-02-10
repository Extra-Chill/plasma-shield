package bastion

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
)

type directTCPIP struct {
	DestAddr string
	DestPort uint32
	OrigAddr string
	OrigPort uint32
}

func handleDirectTCPIP(channel ssh.NewChannel) {
	var payload directTCPIP
	if err := ssh.Unmarshal(channel.ExtraData(), &payload); err != nil {
		channel.Reject(ssh.Prohibited, "invalid direct-tcpip payload")
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
