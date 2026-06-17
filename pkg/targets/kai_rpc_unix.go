//go:build !windows

package targets

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func generatePipePath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("kai-rpc-%d.sock", time.Now().UnixNano()))
}

// waitAndDial waits for the Unix socket to appear, then connects.
func waitAndDial(ctx context.Context, path string, deadline time.Time) (net.Conn, error) {
	// Wait for socket file to exist
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Try to connect with retries
	var conn net.Conn
	var err error
	for time.Now().Before(deadline) {
		conn, err = net.Dial("unix", path)
		if err == nil {
			return conn, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("timed out waiting for socket %s", path)
}

func removePipe(path string) {
	os.Remove(path)
}

func signalGracefulShutdown(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
