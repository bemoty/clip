package cmd

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bemoty/clip/cmd/client/history"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deleteCmd = &cobra.Command{
	Use:          "delete [url-or-id]",
	Short:        "Delete an uploaded file from remote",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		useLast, _ := cmd.Flags().GetBool("last")

		if useLast && len(args) > 0 {
			return fmt.Errorf("--last and a URL argument are mutually exclusive")
		}
		if !useLast && len(args) == 0 {
			return fmt.Errorf("requires a url-or-id argument or --last flag")
		}

		serverURL := viper.GetString("url")
		if serverURL == "" {
			return fmt.Errorf("no server URL configured: set CLIP_URL, use --url, or add url to ~/.config/clip/config.toml")
		}
		server, err := url.Parse(serverURL)
		if err != nil {
			return fmt.Errorf("invalid server URL: %w", err)
		}

		var target string
		if useLast {
			e, err := history.Last()
			if err != nil {
				return err
			}
			if e == nil {
				return fmt.Errorf("history is empty")
			}
			target = e.URL
		} else {
			target = args[0]
		}

		id, err := resolveID(target, server)
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
	deleteCmd.Flags().BoolP("last", "l", false, "Delete the most recently uploaded file")
	RootCmd.AddCommand(deleteCmd)
}
