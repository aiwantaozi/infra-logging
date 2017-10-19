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

	"github.com/aiwantaozi/infra-logging/operator/fluentd"

	_ "github.com/aiwantaozi/infra-logging/provider/fluentd"
)

var VERSION = "v0.0.0-dev"

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
		cli.StringFlag{
			Name:  "kubernete-config",
			Usage: "Kubernete config file path",
		},
	}

	app.Action = func(c *cli.Context) error {
		logrus.Info("Starting Infrastrution Logging")
		sigs := make(chan os.Signal, 1)
		stop := make(chan struct{})
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // Push signals into channel

		wg := &sync.WaitGroup{}
		op, err := fluentd.NewOperator()
		if err != nil {
			return err
		}
		op.Run()

		ct := fluentd.NewController()
		ct.Run()

		<-sigs // Wait for signals (this hangs until a signal arrives)
		glog.Info("Shutting down...")

		close(stop) // Tell goroutines to stop themselves
		wg.Wait()   // Wait for all to be stopped
		return nil
	}

	app.Run(os.Args)
}

func handleSigterm(op fluentd.Operator, ct fluentd.Controller) {
	fmt.Println("in handleSigterm")
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	sig := <-signalChan
	logrus.Infof("Received signal %s", sig.String())

	exitCode := 0

	op.Stop()
	ct.Stop()
	exitCode = 1
	logrus.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)

}
