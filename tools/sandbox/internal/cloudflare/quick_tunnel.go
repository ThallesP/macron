package cloudflare

import (
	"context"
	"fmt"
	"io"
	"regexp"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

const QuickTunnelName = "cloudflare-quick"

type QuickTunnel struct {
	Executable string
}

var quickTunnelHost = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*\.trycloudflare\.com$`)

func (QuickTunnel) Name() string {
	return QuickTunnelName
}

func (t QuickTunnel) Route(endpoint string) (sandbox.Route, error) {
	if !quickTunnelHost.MatchString(endpoint) {
		return sandbox.Route{}, fmt.Errorf("the sandbox announced an address that is not a Cloudflare quick tunnel")
	}
	return sandbox.Route{
		HostName:     endpoint,
		ProxyCommand: fmt.Sprintf("%s proxy %s %%h", sandbox.QuoteForShell(t.Executable), QuickTunnelName),
	}, nil
}

func ProxyQuickTunnel(ctx context.Context, host string, in io.Reader, out io.Writer) error {
	if !quickTunnelHost.MatchString(host) {
		return fmt.Errorf("%q is not a Cloudflare quick tunnel address", host)
	}

	tunnel, err := dialWebSocket(ctx, host)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	finished := make(chan error, 2)
	go func() {
		_, err := io.Copy(tunnel, in)
		finished <- err
	}()
	go func() {
		_, err := io.Copy(out, tunnel)
		finished <- err
	}()
	return <-finished
}
