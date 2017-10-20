package fluentd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	loggingv1 "github.com/aiwantaozi/infra-logging/client/logging/v1"
	infraconfig "github.com/aiwantaozi/infra-logging/config"
	"github.com/aiwantaozi/infra-logging/provider"
)

const (
	ConfigFile   = "fluentd.conf"
	TmpFile      = "tmp.conf"
	PidFile      = "fluentd.pid"
	TemplatePath = "/fluentd/etc/fluentd_template.conf"
	PluginsPath  = "fluentd/etc/plugins"
)

var (
	fluentdProcess *exec.Cmd
	cfgPath        string
	pidPath        string
	tmpPath        string
)

type Provider struct {
	cfg    fluentdConfig
	stopCh chan struct{}
}

//TODO change to lowercase
type fluentdConfig struct {
	Name      string
	StartCmd  string
	ConfigDir string
}

func init() {
	logp := Provider{
		cfg:    fluentdConfig{Name: "fluentd"},
		stopCh: make(chan struct{}),
	}
	provider.RegisterProvider(logp.GetName(), &logp)
}

func (logp *Provider) Init(c *cli.Context) {
	logp.cfg.ConfigDir = c.String("fluentd-config-dir")
	cfgPath = path.Join(logp.cfg.ConfigDir, ConfigFile)
	pidPath = path.Join(logp.cfg.ConfigDir, PidFile)
	tmpPath = path.Join(logp.cfg.ConfigDir, TmpFile)
	logp.cfg.StartCmd = "fluentd " + "-c " + cfgPath + " -p " + PluginsPath + " -d " + pidPath
}

func (logp *Provider) GetName() string {
	return "fluentd"
}

func (logp *Provider) Run() {
	cfg, err := infraconfig.GetLoggingConfig(loggingv1.Namespace, loggingv1.LoggingName)
	if err != nil {
		logrus.Errorf("Error in StartFluentd get logging config, details: %s", err.Error())
		<-logp.stopCh
		return
	}
	if err = logp.cfg.write(cfg); err != nil {
		logrus.Errorf("Error in StartFluentd write config, details: %s", err.Error())
		<-logp.stopCh
		return
	}

	if err := logp.StartFluentd(); err != nil {
		logrus.Errorf("Error in StartFluentd, details: %s", err.Error())
		<-logp.stopCh
		return
	}
	<-logp.stopCh
}

func (logp *Provider) Stop() error {
	logrus.Infof("Shutting down provider %v", logp.GetName())
	close(logp.stopCh)
	return nil
}

func (logp *Provider) StartFluentd() error {
	return logp.cfg.start()
}

func (logp *Provider) Reload() error {
	return logp.cfg.reload()
}

func (logp *Provider) ApplyConfig(infraCfg infraconfig.InfraLoggingConfig) error {
	err := logp.cfg.write(infraCfg)
	if err != nil {
		return err
	}
	err = logp.cfg.reload()
	if err != nil {
		return err
	}
	return nil
}

func (cfg *fluentdConfig) start() error {
	//TODO: graceful way to run command, handle fluent process shut down
	fluentdProcess = exec.Command("sh", "-c", cfg.StartCmd)
	output, err := fluentdProcess.CombinedOutput()
	msg := fmt.Sprintf("%v -- %v", cfg.Name, string(output))
	if string(output) != "" {
		logrus.Info(msg)
	}
	logrus.Debug("After Running start command")
	if err != nil {
		return fmt.Errorf("error starting %v, details: %v", msg, err)
	}
	return nil
}

func (cfg *fluentdConfig) reload() error {
	pidFile, err := ioutil.ReadFile(pidPath)
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(pidFile)))
	if err != nil {
		return fmt.Errorf("error parsing pid from %s: %s", pidFile, err)
	}

	if pid <= 0 {
		logrus.Warning("Fluentd not start yet, could not reload")
		return nil
	}
	if _, err := os.FindProcess(pid); err != nil {
		return fmt.Errorf("error find process pid: %d, details: %v", pid, err)
	}

	if err = syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("error reloading, details: %v", err)
	}
	return nil
}

func (cfg *fluentdConfig) write(infraCfg infraconfig.InfraLoggingConfig) (err error) {
	var w io.Writer

	w, err = os.Create(tmpPath)
	if err != nil {
		return errors.Wrap(err, "fluentd create temp config file state error")
	}

	if _, err := os.Stat(tmpPath); err != nil {
		return errors.Wrap(err, "fluentd temp config file state error")
	}

	var t *template.Template
	t, err = template.ParseFiles(TemplatePath)
	if err != nil {
		return err
	}
	conf := make(map[string]interface{})
	conf["stores"] = infraCfg.Targets
	conf["sources"] = infraCfg.Sources
	err = t.Execute(w, conf)
	if err != nil {
		return err
	}

	from, err := os.Open(tmpPath)
	if err != nil {
		return errors.Wrap(err, "fail to open tmp config file")
	}
	defer from.Close()

	to, err := os.OpenFile(cfgPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return errors.Wrap(err, "fail to open current config file")
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return errors.Wrap(err, "fail to copy config file")
	}
	return err
}
