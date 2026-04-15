package tcp

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
)

// ListenAndServe starts the TCP server for the CSIL protocol.
// This is a placeholder that accepts connections and logs them.
// Full CSIL protocol handling will be implemented after csilgen integration.
func ListenAndServe(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp listen: %w", err)
	}
	defer listener.Close()

	log.Infof("TCP/CSIL server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.WithError(err).Error("TCP accept error")
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Infof("TCP connection from %s", conn.RemoteAddr())
	// TODO: Implement CSIL protocol handling after csilgen code generation
	conn.Write([]byte("longhouse CSIL protocol - not yet implemented\n"))
}
