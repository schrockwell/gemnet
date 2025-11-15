package server

import (
	"log"
	"net"

	"gemnet/internal/session"
)

func HandleConnection(conn net.Conn) {
	defer conn.Close()

	sess := session.New(conn)
	if err := sess.Run(); err != nil {
		log.Printf("Session error: %v\n", err)
	}
}
