//go:build windows

package targets

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
)

func generatePipePath() string {
	return fmt.Sprintf(`\\.\pipe\kai-rpc-%d`, time.Now().UnixNano())
}

// waitAndDial connects to a Windows named pipe, retrying until the deadline.
// Named pipes can't be stat'd, so we just retry dialing directly.
func waitAndDial(ctx context.Context, path string, deadline time.Time) (net.Conn, error) {
	var conn net.Conn
	var err error
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining > 5*time.Second {
			remaining = 5 * time.Second
		}
		conn, err = winio.DialPipe(path, &remaining)
		if err == nil {
			return conn, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// dialPipe already waited; retry immediately
		}
	}
	return nil, err
}

func removePipe(_ string) {
	// Named pipes are cleaned up when the last handle closes.
}

func signalGracefulShutdown(p *os.Process) error {
	// Windows has no SIGTERM; kill the process directly.
	return p.Kill()
}
