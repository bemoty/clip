package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/bemoty/clip/cmd/client/history"
	"github.com/gen2brain/beeep"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var RootCmd = &cobra.Command{
	Use:   binaryName,
	Short: "Upload your clipboard for sharing",
	Long:  "clip uploads your clipboard to a server and returns a shareable link for use with services like IRC or TS3",
	Example: `clip note.txt
clip note.txt --ttl 7d
cat main.go | clip -l go`,
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var r io.Reader
		var filename, source string
		var size int64 = -1

		switch {
		case len(args) > 0 && args[0] != "-":
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer func(f *os.File) {
				_ = f.Close()
			}(f)
			if info, err := f.Stat(); err == nil {
				size = info.Size()
			}
			r = f
			filename = args[0]
			source = filepath.Base(args[0])
		case len(args) > 0 && args[0] == "-":
			r = os.Stdin
			source = "stdin"
		default:
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				r = os.Stdin
				source = "stdin"
			} else {
				text, err := clipboard.ReadAll()
				if err != nil {
					return err
				}
				size = int64(len(text))
				r = strings.NewReader(text)
				source = "clipboard"
			}
		}

		return upload(cmd, r, filename, source, size)
	},
}

func upload(cmd *cobra.Command, r io.Reader, filename, source string, size int64) error {
	contentType := "application/octet-stream"
	if ext := filepath.Ext(filename); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			contentType = t
		}
	}
	if contentType == "application/octet-stream" {
		head := make([]byte, 512)
		n, _ := r.Read(head)
		head = head[:n]
		if looksLikeText(head) {
			contentType = "text/plain; charset=utf-8"
		} else {
			contentType = http.DetectContentType(head)
		}
		r = io.MultiReader(bytes.NewReader(head), r)
	}
	pr := NewProgressReader(r, size)
	defer pr.Done()
	body := pr

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
	if ttl, _ := cmd.Flags().GetString("ttl"); ttl != "" {
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
	pr.Done()
	fmt.Println(uploadedURL)

	if viper.GetBool("history") {
		ttl, _ := cmd.Flags().GetString("ttl")
		if err := history.Append(history.Entry{
			Ts:     time.Now().UTC(),
			URL:    uploadedURL,
			Source: source,
			TTL:    ttl,
		}); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "warning: could not write history:", err)
		}
	}

	if cp, _ := cmd.Flags().GetBool("copy"); cp {
		if err := clipboard.WriteAll(uploadedURL); err != nil {
			return err
		}
		err := beeep.Notify(binaryName, "Link copied to clipboard", "")
		if err != nil {
			return err
		}
	}

	if open, _ := cmd.Flags().GetBool("open"); open {
		var c *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			c = exec.Command("xdg-open", uploadedURL)
		case "darwin":
			c = exec.Command("open", uploadedURL)
		case "windows":
			c = exec.Command("cmd", "/c", "start", uploadedURL)
		}
		if c != nil {
			_ = c.Start()
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
	beeep.AppName = binaryName
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().String("url", "", "Overrides the configured address of the server to upload to")
	RootCmd.PersistentFlags().String("key", "", "Overrides the configured auth key for the server to upload to")

	RootCmd.Flags().StringP("lang", "l", "", "Programming language of the uploaded code (only has effect for text upload)")
	RootCmd.Flags().String("ttl", "", "Time to live of the uploaded file")
	RootCmd.Flags().BoolP("copy", "c", false, "Copies the shareable link to clipboard and outputs a notification")
	RootCmd.Flags().BoolP("open", "o", false, "Opens the shareable link in the default browser")
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

	viper.SetDefault("history", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			_, _ = fmt.Fprintln(os.Stderr, "could not read config:", err)
			os.Exit(1)
		}
	}
}
