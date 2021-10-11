package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/gosuri/uitable"
	pkggodevclient "github.com/guseggert/pkggodev-client"
	"github.com/logrusorgru/aurora/v3"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var rootCmd = cobra.Command{
	Use:   "pkggodev",
	Short: "CLI interface for pkg.go.dev",
}

var stdoutIsTerminal = isatty.IsTerminal(os.Stdout.Fd())

func init() {
	var format string
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "pretty", "pretty|json")

	rootCmd.AddCommand(&cobra.Command{
		Use:           "imported-by package [packages...]",
		Short:         "show the packages that import the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			importedBy, err := client.ImportedBy(pkggodevclient.ImportedByRequest{
				Package: args[0],
			})
			if err != nil {
				return err
			}
			err = printOutput(format, importedBy.ImportedBy)
			if err != nil {
				return err
			}
			return nil
		},
	})

	var searchLimit int
	searchCmd := &cobra.Command{
		Use:           "search query",
		Short:         "search for packages",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			client := pkggodevclient.New()
			res, err := client.Search(pkggodevclient.SearchRequest{
				Query: query,
				Limit: searchLimit,
			})
			if err != nil {
				return err
			}
			return printOutput(format, res.Results)
		},
	}
	searchCmd.Flags().IntVar(&searchLimit, "limit", 25, "")
	rootCmd.AddCommand(searchCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:           "versions package [package]...",
		Short:         "show version information for the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			versions, err := client.Versions(pkggodevclient.VersionsRequest{
				Package: args[0],
			})
			if err != nil {
				return err
			}
			err = printOutput(format, versions.Versions)
			if err != nil {
				return err
			}
			return nil
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:           "package-info package [package]...",
		Short:         "show package information for the given package(s)",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := pkggodevclient.New()
			for _, pkg := range args {
				d, err := client.DescribePackage(pkggodevclient.DescribePackageRequest{
					Package: pkg,
				})
				if err != nil {
					return err
				}
				err = printOutput(format, d)
				if err != nil {
					return err
				}
			}
			return nil
		},
	})
}

func printStruct(v reflect.Value) {
	// assume there are no nested structs
	table := uitable.New()
	table.MaxColWidth = 80
	table.Wrap = true
	for fieldNum := 0; fieldNum < v.NumField(); fieldNum++ {
		fieldName := v.Type().Field(fieldNum).Name
		field := v.Field(fieldNum)
		fieldValStr := fmt.Sprintf("%v", field)

		if stdoutIsTerminal {
			table.AddRow(aurora.Bold(fieldName+":"), fieldValStr)
		} else {
			table.AddRow(fieldName+":", fieldValStr)
		}
	}
	os.Stdout.WriteString(table.String() + "\n")
}

func printSlice(v reflect.Value) {
	elemKind := v.Type().Elem().Kind()

	for i := 0; i < v.Len(); i++ {
		if elemKind == reflect.Struct {
			printStruct(v.Index(i))
		} else {
			fmt.Fprintf(os.Stdout, "%v", v.Index(i))
		}
		os.Stdout.WriteString("\n")
	}
}

func printOutput(format string, v interface{}) error {
	switch format {
	case "json":
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("formatting JSON output: %w", err)
		}
		fmt.Println(string(b))
	case "pretty":
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Ptr:
			printOutput(format, rv.Elem().Interface())
		case reflect.Slice:
			printSlice(rv)
		case reflect.Struct:
			printStruct(rv)
		default:
			return fmt.Errorf("unable to pretty print for value with type '%s'", rv.Type().String())
		}
	default:
		return fmt.Errorf("unknown format type '%s'", format)
	}
	return nil
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
