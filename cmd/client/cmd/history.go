package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bemoty/clip/cmd/client/history"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var historyCmd = &cobra.Command{
	Use:          "history",
	Short:        "Show upload history",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !viper.GetBool("history") {
			return fmt.Errorf("history is disabled in config")
		}

		entries, err := history.ReadAll()
		if err != nil {
			return err
		}

		if n, _ := cmd.Flags().GetInt("last"); n > 0 && n < len(entries) {
			entries = entries[len(entries)-n:]
		}

		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			for _, e := range entries {
				line, err := json.Marshal(e)
				if err != nil {
					return err
				}
				fmt.Println(string(line))
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, e := range entries {
			ts := e.Ts.Local().Format("2006-01-02 15:04")
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ts, e.Source, e.URL, e.TTL)
		}
		return w.Flush()
	},
}

var historyPathCmd = &cobra.Command{
	Use:          "path",
	Short:        "Print the path to history.jsonl",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := history.Path()
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

func init() {
	historyCmd.Flags().IntP("last", "n", 0, "Show only the last N entries")
	historyCmd.Flags().Bool("json", false, "Emit raw JSONL")

	historyCmd.AddCommand(historyPathCmd)
	RootCmd.AddCommand(historyCmd)
}
