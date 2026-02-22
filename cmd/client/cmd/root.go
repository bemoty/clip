package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gen2brain/beeep"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:   "clip",
	Short: "Upload your clipboard for sharing",
	Long:  "clip uploads your clipboard to a server and returns a shareable link for use with services like IRC or TS3",
	Example: `clip note.txt
clip note.txt --ttl 7d
cat main.go | clip -l go`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var r io.Reader

		switch {
		case len(args) > 0 && args[0] != "-":
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer func(f *os.File) {
				_ = f.Close()
			}(f)
			r = f
		case len(args) > 0 && args[0] == "-":
			r = os.Stdin
		default:
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				r = os.Stdin
			} else {
				text, err := clipboard.ReadAll()
				if err != nil {
					return err
				}
				r = strings.NewReader(text)
			}
		}

		return upload(cmd, r)
	},
}

func upload(cmd *cobra.Command, r io.Reader) error {
	head := make([]byte, 512)
	n, err := r.Read(head)
	if err != nil && err != io.EOF {
		return err
	}
	head = head[:n]

	contentType := http.DetectContentType(head)
	if contentType == "application/octet-stream" && looksLikeText(head) {
		contentType = "text/plain; charset=utf-8"
	}
	body := io.MultiReader(bytes.NewReader(head), r)

	serverURL := viper.GetString("url")
	if serverURL == "" {
		return fmt.Errorf("no server URL configured: set CLIP_URL, use --url, or add url to ~/.config/clip/config.toml")
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	q := u.Query()
	if lang, _ := cmd.Flags().GetString("lang"); lang != "" {
		q.Set("lang", lang)
	}
	if ttl := viper.GetString("ttl"); ttl != "" {
		q.Set("ttl", ttl)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if key := viper.GetString("key"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("server returned %d: %s", res.StatusCode, strings.TrimSpace(string(msg)))
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	uploadedURL := strings.TrimSpace(string(resBody))
	fmt.Println(uploadedURL)

	if cp, _ := cmd.Flags().GetBool("copy"); cp {
		if err := clipboard.WriteAll(uploadedURL); err != nil {
			return err
		}
		err := beeep.Notify("clip", "Link copied to clipboard", "")
		if err != nil {
			return err
		}
	}
	return nil
}

func looksLikeText(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	beeep.AppName = "clip"
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().String("url", "", "Overrides the configured address of the server to upload to")
	RootCmd.PersistentFlags().String("key", "", "Overrides the configured auth key for the server to upload to")

	RootCmd.Flags().StringP("lang", "l", "", "Programming language of the uploaded code (only has effect for text upload)")
	RootCmd.Flags().String("ttl", "", "Time to live of the uploaded file")
	RootCmd.Flags().BoolP("copy", "c", false, "Copies the shareable link to clipboard and outputs a notification")
}

func initConfig() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(filepath.Join(configDir, "clip"))

	viper.SetEnvPrefix("CLIP")
	viper.AutomaticEnv()

	if err := viper.BindPFlag("url", RootCmd.PersistentFlags().Lookup("url")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("key", RootCmd.PersistentFlags().Lookup("key")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("ttl", RootCmd.Flags().Lookup("ttl")); err != nil {
		panic(err)
	}

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			_, _ = fmt.Fprintln(os.Stderr, "could not read config:", err)
			os.Exit(1)
		}
	}
}
