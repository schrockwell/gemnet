package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	port := ":2323"
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Printf("gemnet telnet server listening on %s\n", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	session := NewSession(conn)
	if err := session.Run(); err != nil {
		log.Printf("Session error: %v\n", err)
	}
}
