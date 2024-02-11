//go:build windows

package agent

import (
	"net"
	"time"

	npipe "gopkg.in/natefinch/npipe.v2"
)

// dialWithTimeout dials the named pipe with a timeout
func dialWithTimeout(socketPath string, timeout time.Duration) (net.Conn, error) {
	return npipe.DialTimeout(socketPath, timeout)
}

// listen listens on the named pipe
func listen(socketPath string) (net.Listener, error) {
	return npipe.Listen(socketPath)
}
