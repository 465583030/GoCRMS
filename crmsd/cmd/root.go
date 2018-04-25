package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
	"os"
	"time"
	"runtime"
	"github.com/WenzheLiu/GoCRMS/gocrms"
	"github.com/coreos/etcd/clientv3"
	"log"
	"path"
)

const (
	defaultDialTimeout      = 2 * time.Second
	defaultRequestTimeout   = 10 * time.Second
	defaultCommandTimeOut   = 5 * time.Second
	defaultKeepAliveTime    = 2 * time.Second
	defaultKeepAliveTimeOut = 6 * time.Second
)

var rootCmd = &cobra.Command{
	Use:   "crmsd",
	Short: "crmsd is a server to run crms jobs",
	Long: `crmsd is a server to run crms jobs, based on etcd`,
	Run: func(cmd *cobra.Command, args []string) {
		gocrms.MkDataDir()

		flag := log.Ldate | log.Lmicroseconds
		if globalFlags.Debug {
			flag |= log.Lshortfile
		}
		logfile := gocrms.InitServerLog(globalFlags.Name, flag, globalFlags.LogToStdOut)

		defer logfile.Close()


		crmsd, err := gocrms.NewCrmsServer(clientv3.Config{
			Endpoints:globalFlags.Endpoints,
			DialTimeout:globalFlags.DialTimeout,
		}, globalFlags.RequestTimeout, gocrms.Server{
			Name:globalFlags.Name,
			SlotCount:globalFlags.SlotCount,
		})
		if err != nil {
			log.Panicln(err)
		}
		defer crmsd.Close()
		if err := crmsd.Start(); err != nil {
			log.Panicln(err)
		}
		log.Println("CRMS server", globalFlags.Name, "has started with", globalFlags.SlotCount, "slots.")
		crmsd.WaitUntilClose()
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
		&globalFlags.Endpoints, "endpoints", []string{"127.0.0.1:2379"}, "gRPC endpoints")
	rootCmd.PersistentFlags().BoolVar(
		&globalFlags.Debug, "debug", false, "enable client-side debug logging")
	rootCmd.PersistentFlags().BoolVar(&globalFlags.LogToStdOut, "stdout", false, "log to stdout as well")
	rootCmd.PersistentFlags().StringVarP(
		&gocrms.DataDir, "data-dir", "d", path.Join(os.Getenv("HOME"), ".gocrms"), "set the data directory")
	rootCmd.PersistentFlags().StringVarP(
		&globalFlags.OutputFormat, "write-out", "w", "simple", "set the output format (simple, json, json_compact)")
	rootCmd.PersistentFlags().DurationVar(
		&globalFlags.DialTimeout, "dial-timeout", defaultDialTimeout, "dial timeout for client connections")
	rootCmd.PersistentFlags().DurationVar(
		&globalFlags.RequestTimeout, "request-timeout", defaultRequestTimeout, "request timeout for client connections")

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "no_host_name"
	}

	rootCmd.PersistentFlags().StringVar(&globalFlags.Name, "name", hostName, "Server name, default is the host name")
	rootCmd.PersistentFlags().IntVar(&globalFlags.SlotCount, "slot", runtime.NumCPU(), "Slot count, default is number of CPU")
}
