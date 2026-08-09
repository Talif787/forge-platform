package temporal

import (
	"fmt"

	"go.temporal.io/sdk/client"
)

// BuildClient returns a lazy Temporal client: it does not connect until the
// first request, so a Temporal outage never blocks API startup. Per-request
// failures are surfaced by the reader as 503, keeping the provisioning view
// optional and non-fatal.
func BuildClient(hostPort, namespace string) (client.Client, error) {
	c, err := client.NewLazyClient(client.Options{HostPort: hostPort, Namespace: namespace})
	if err != nil {
		return nil, fmt.Errorf("build temporal client: %w", err)
	}
	return c, nil
}
