package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const healthCheckTimeout = 5 * time.Second

// newHealthCmd creates a cobra command that checks the health of a running mimir instance.
// It performs an HTTP GET request to the /livez endpoint and reports whether the server is healthy.
func newHealthCmd() *cobra.Command {
	var url string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check the health of a running mimir instance",
		Long:  "Perform a health check against a running mimir instance by querying the /livez endpoint.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHealthCheck(cmd.Context(), url)
		},
	}

	cmd.Flags().StringVar(&url, "url", "http://localhost:7000", "base URL of the mimir instance")

	return cmd
}

// runHealthCheck performs an HTTP GET to the /livez endpoint at the given base URL.
// If the server responds with HTTP 200 it decodes the JSON body and prints a
// human-readable summary. Non-200 responses are returned as errors.
func runHealthCheck(ctx context.Context, baseURL string) error {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	endpoint := strings.TrimRight(baseURL, "/") + "/livez"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Fallback for older servers that return plain text.
	fmt.Fprintln(os.Stdout, "Ok") //nolint:forbidigo // CLI output is intentional

	return nil
}
