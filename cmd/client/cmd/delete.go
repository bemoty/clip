package cmd

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <url-or-id>",
	Short: "Delete an uploaded file from remote",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := viper.GetString("url")
		if serverURL == "" {
			return fmt.Errorf("no server URL configured: set CLIP_URL, use --url, or add url to ~/.config/clip/config.toml")
		}
		server, err := url.Parse(serverURL)
		if err != nil {
			return fmt.Errorf("invalid server URL: %w", err)
		}

		id, err := resolveID(args[0], server)
		if err != nil {
			return err
		}

		req, err := http.NewRequest(http.MethodDelete, serverURL+"/"+id, nil)
		if err != nil {
			return err
		}
		if key := viper.GetString("key"); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = res.Body.Close() }()

		if res.StatusCode == http.StatusNotFound {
			return fmt.Errorf("not found: may have already expired or been deleted")
		}
		if res.StatusCode/100 != 2 {
			return fmt.Errorf("server returned %d: %s", res.StatusCode, res.Status)
		}

		return nil
	},
}

func resolveID(arg string, server *url.URL) (string, error) {
	u, err := url.Parse(arg)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return arg, nil
	}
	if u.Host != server.Host {
		return "", fmt.Errorf("URL hostname %q differs from configured server %q\n  use --url %s to target a different server", u.Host, server.Host, u.Host)
	}
	return u.Path[1:], nil
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}
