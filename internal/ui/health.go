// Copyright 2023 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"lucor.dev/paw/internal/paw"
)

const (
	timeout = 100 * time.Millisecond
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

// HealthService starts a health service that listens on a random port.
// In the current implementation is used only to avoid starting multiple
// instances of the app.
func HealthService(lockFile string) (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Println("could not start health service:", err)
		return nil, err
	}
	defer listener.Close()

	err = os.WriteFile(lockFile, []byte(listener.Addr().String()), 0644)
	if err != nil {
		log.Println("could not write health service lock file:", err)
		return nil, err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return listener, fmt.Errorf("health service: error accepting connection: %w", err)
		}
		go handleConnection(conn)
	}
}

// HealthServiceCheck checks if the health service is running.
func HealthServiceCheck(lockFile string) bool {
	address, err := os.ReadFile(lockFile)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return false
	}

	// Read the service version from the app and close the connection
	conn.SetReadDeadline(time.Now().UTC().Add(timeout))
	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	conn.Close()
	if err != nil {
		return false
	}

	// check for paw service
	return bytes.Equal([]byte(paw.ServicePrefix), buffer)
}
