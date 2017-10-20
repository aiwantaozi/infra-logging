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
	"time"

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
	logFile      = "fluentd.log"
)

var (
	fluentdProcess *exec.Cmd
	cfgPath        string
	cfgPathBak     string
	pidPath        string
	tmpPath        string
	logPath        string
	fluentdTimeout = 1 * time.Minute
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
	cfgPathBak = path.Join(logp.cfg.ConfigDir, ConfigFile+".bak")
	pidPath = path.Join(logp.cfg.ConfigDir, PidFile)
	tmpPath = path.Join(logp.cfg.ConfigDir, TmpFile)
	logPath = path.Join(logp.cfg.ConfigDir, logFile)
	logp.cfg.StartCmd = "fluentd " + "-c " + cfgPath + " -p " + PluginsPath + " -d " + pidPath + " --log " + logPath
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
	cmd := exec.Command("sh", "-c", cfg.StartCmd)

	var buf bytes.Buffer
	cmd.Stdout = &buf

	cmd.Start()

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	timeout := time.After(fluentdTimeout)

	select {
	case <-timeout:
		cmd.Process.Kill()
		return errors.New("Fluentd command timed out")
	case err := <-done:
		logrus.Error("Fluentd Output:", buf.String())
		if err != nil {
			logrus.Error("Fluentd return a Non-zero exit code:", err)
			return err
		}
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
	err = os.Rename(cfgPath, cfgPathBak)
	if err != nil {
		return errors.Wrap(err, "fail to rename config config file")
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
	if err = to.Sync(); err != nil {
		return errors.Wrap(err, "fail to sync config file")
	}
	return nil
}
