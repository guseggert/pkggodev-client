package main

import (
	"encoding/json"
	"fmt"
	"os"

	pkggodevclient "github.com/guseggert/pkggodev-client"
	"github.com/spf13/cobra"
)

var rootCmd = cobra.Command{
	Use:   "pkggodev",
	Short: "CLI interface for pkg.go.dev",
}

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:           "imported-by package [packages...]",
		Short:         "show the packages that import the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			for _, pkg := range args {
				importedBy, err := client.ImportedBy(pkg)
				if err != nil {
					return err
				}
				b, err := json.Marshal(importedBy)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			}
			return nil
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:           "search query",
		Short:         "search for packages",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			client := pkggodevclient.New()
			res, err := client.Search(query)
			if err != nil {
				return err
			}
			b, err := json.Marshal(res)
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:           "versions package [packages...]",
		Short:         "show version information for the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			for _, pkg := range args {
				versions, err := client.Versions(pkg)
				if err != nil {
					return err
				}
				b, err := json.Marshal(versions)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			}
			return nil
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:           "package-info package [packages...]",
		Short:         "show package information for the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			for _, pkg := range args {
				d, err := client.DescribePackage(pkg)
				if err != nil {
					return err
				}
				b, err := json.Marshal(d)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			}
			return nil
		},
	})
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
