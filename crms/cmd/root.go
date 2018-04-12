package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
	"os"
	"time"
)

const (
	defaultDialTimeout      = 2 * time.Second
	defaultRequestTimeout   = 10 * time.Second
	defaultCommandTimeOut   = 5 * time.Second
	defaultKeepAliveTime    = 2 * time.Second
	defaultKeepAliveTimeOut = 6 * time.Second
)

var rootCmd = &cobra.Command{
	Use:   "crms",
	Short: "crms is a client to visit GoCRMS cluster",
	Long: `crms is a client to visit GoCRMS cluster, based on etcd`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringSliceVar(
		&global.Endpoints, "endpoints", []string{"127.0.0.1:2379"}, "gRPC endpoints")
	rootCmd.PersistentFlags().BoolVar(
		&global.Debug, "debug", false, "enable client-side debug logging")
	rootCmd.PersistentFlags().StringVarP(
		&global.OutputFormat, "write-out", "w", "simple", "set the output format")
	rootCmd.PersistentFlags().DurationVar(
		&global.DialTimeout, "dial-timeout", defaultDialTimeout, "dial timeout for client connections")
	rootCmd.PersistentFlags().DurationVar(
		&global.RequestTimeout, "request-timeout", defaultRequestTimeout, "request timeout for client connections")
}
