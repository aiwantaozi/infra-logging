package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/urfave/cli"

	infraCfg "github.com/aiwantaozi/infra-logging/config"
	"github.com/aiwantaozi/infra-logging/operator/fluentd"
	"github.com/aiwantaozi/infra-logging/provider"
	_ "github.com/aiwantaozi/infra-logging/provider/fluentd"
)

var VERSION = "v0.0.0-dev"

//TODO:
//1. remove useless k8s.io code
//2. package a better base image

var (
	logControllerName string
	logProviderName   string
	metadataAddress   string
)

func main() {
	app := cli.NewApp()
	app.Name = "infra-logging"
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "fluentd-dry-run",
			Usage: "generate the config file, but not run fluentd",
		},
		cli.StringFlag{
			Name:  "fluentd-config-dir",
			Usage: "Fluentd config directory",
			Value: "/fluentd/etc/config",
		},
		cli.StringFlag{
			Name:  "fluentd-plugin-dir",
			Usage: "Fluentd plugin directory",
			Value: "/fluentd/etc/plugins",
		},
		cli.StringFlag{
			Name:  "k8s-config-path",
			Usage: "k8s config path",
		},
	}

	app.Action = func(c *cli.Context) error {
		logrus.Info("Starting Infrastrution Logging")
		sigs := make(chan os.Signal, 1)
		stop := make(chan struct{})
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // Push signals into channel

		infraCfg.Init(c)
		if err := infraCfg.IsReachable(); err != nil {
			return err
		}

		wg := &sync.WaitGroup{}
		prd := provider.GetProvider("fluentd", c)
		go prd.Run()

		op, err := fluentd.NewOperator(prd)
		if err != nil {
			return err
		}

		if err = op.Run(); err != nil {
			logrus.Errorf("Error run operator, details: %v", err)
			return err
		}

		//TODO: better stop chan handle
		go handleSigterm(op)
		<-sigs // Wait for signals (this hangs until a signal arrives)
		glog.Info("Shutting down...")

		close(stop) // Tell goroutines to stop themselves
		wg.Wait()   // Wait for all to be stopped
		return nil
	}

	app.Run(os.Args)
}

func handleSigterm(op *fluentd.Operator) {
	fmt.Println("in handleSigterm")
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	sig := <-signalChan
	logrus.Infof("Received signal %s", sig.String())

	exitCode := 0

	op.Stop()
	exitCode = 1
	logrus.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)

}
