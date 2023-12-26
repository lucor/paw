package ui

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"lucor.dev/paw/internal/paw"
)

const (
	startPortRange = 54321
	endPortRange   = 55000
	timeout        = 100 * time.Millisecond
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
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			continue
		}

		// Read the service version from the app and close the connection
		conn.SetReadDeadline(time.Now().Add(timeout))
		buffer := make([]byte, 4)
		_, err = conn.Read(buffer)
		conn.Close()
		if err != nil {
			// error reading
			continue
		}

		// check for paw service
		if bytes.Equal([]byte(paw.ServicePrefix), buffer) {
			return true
		}
	}

	return false
}
