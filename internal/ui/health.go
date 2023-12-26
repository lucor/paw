package ui

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"

	"lucor.dev/paw/internal/paw"
)

const (
	startPortRange = 54321
	endPortRange   = 55000
)

// handleConnection handles the connection returning the paw version
func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send service information to the client and exits
	_, err := conn.Write([]byte(paw.ServiceVersion() + "\n"))
	if err != nil {
		fmt.Println("Error writing server info:", err)
		return
	}
}

// HealthService
func HealthService() (net.Listener, error) {
	var listener net.Listener
	var err error
	var address string
	for i := startPortRange; i < endPortRange; i++ {
		address = fmt.Sprintf("127.0.0.1:%d", i)
		listener, err = net.Listen("tcp", address)
		if err == nil {
			defer listener.Close()
			break
		}
		log.Println("health service: error listening:", err)
	}

	if listener == nil {
		return listener, errors.New("health service: could not start")
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return listener, fmt.Errorf("health service: error accepting connection: %w", err)
		}
		go handleConnection(conn)
	}
}

func HealthServiceCheck() bool {
	var address string
	for i := startPortRange; i < endPortRange; i++ {
		address = fmt.Sprintf("127.0.0.1:%d", i)

		conn, err := net.Dial("tcp", address)
		if err != nil {
			continue
		}
		defer conn.Close()

		// Read the service version from the app
		buffer := make([]byte, 4)
		_, err = conn.Read(buffer)
		if err != nil {
			// error reading
			continue
		}

		// check for paw service
		if bytes.Equal([]byte(paw.ServicePrefix), buffer) {
			return true
		}
		continue
	}

	return false
}
