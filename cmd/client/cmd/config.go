package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage clip configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath()
		if err != nil {
			return err
		}
		if err := ensureConfig(path); err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

var configOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the config file in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath()
		if err != nil {
			return err
		}
		if err := ensureConfig(path); err != nil {
			return err
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			return fmt.Errorf("$EDITOR is not set")
		}
		c := exec.Command(editor, path)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath()
		if err != nil {
			return err
		}
		if err := ensureConfig(path); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

func configPath() (string, error) {
	if p := viper.ConfigFileUsed(); p != "" {
		return p, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "clip", "config.toml"), nil
}

func ensureConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := viper.WriteConfigAs(path); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configOpenCmd)
	configCmd.AddCommand(configShowCmd)

	RootCmd.AddCommand(configCmd)
}
