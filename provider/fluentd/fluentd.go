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
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	infraconfig "github.com/aiwantaozi/infra-logging/config"
	"github.com/aiwantaozi/infra-logging/provider"
)

const (
	ConfigDir    = "/Users/fengcaixiao/Desktop/work/src/github.com/aiwantaozi/infra-logging/provider/fluentd/config"
	ConfigFile   = "fluentd.conf"
	TmpFile      = "tmp.conf"
	TemplateFile = "fluentd_template.conf"
	PidFile      = "fluentd.pid"
)

var (
	fluentdProcess *exec.Cmd
)

type Provider struct {
	cfg    fluentdConfig
	stopCh chan struct{}
}

type fluentdConfig struct {
	Name               string
	StartCmd           string
	ConfigPath         string
	TmpConfigPath      string
	TemplateConfigPath string
	PidPath            string
}

func init() {
	cfg := fluentdConfig{
		ConfigPath:         path.Join(ConfigDir, ConfigFile),
		TmpConfigPath:      path.Join(ConfigDir, TmpFile),
		TemplateConfigPath: path.Join(ConfigDir, TemplateFile),
		PidPath:            path.Join(ConfigDir, PidFile),
	}
	cfg.StartCmd = "fluentd -c " + cfg.ConfigPath + " -p /fluentd/plugins -d " + cfg.PidPath
	logp := Provider{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	provider.RegisterProvider(logp.GetName(), &logp)
}

func (logp *Provider) GetName() string {
	return "fluentd"
}

func (logp *Provider) Run() {
	if err := logp.StartFluentd(); err != nil {
		logrus.Errorf("Error in StartFluentd, details: %s", err.Error())
		<-logp.stopCh
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
	fluentdProcess = exec.Command("sh", "-c", cfg.StartCmd)
	output, err := fluentdProcess.CombinedOutput()
	msg := fmt.Sprintf("%v -- %v", cfg.Name, string(output))
	if string(output) != "" {
		logrus.Info(msg)
	}
	if err != nil {
		return fmt.Errorf("error starting %v, details: %v", msg, err)
	}
	return nil
}

func (cfg *fluentdConfig) reload() error {
	pidFile, err := ioutil.ReadFile(cfg.PidPath)
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(pidFile)))
	logrus.Warning("Influentd reload pid: %d, err: %v", pid, err)
	// if err != nil {
	// 	return fmt.Errorf("error parsing pid from %s: %s", pidFile, err)
	// }
	// if _, err := os.FindProcess(pid); err != nil {
	// 	return fmt.Errorf("error find process pid: %d, details: %v", pid, err)
	// }

	// if err = syscall.Kill(pid, syscall.SIGHUP); err != nil {
	// 	return fmt.Errorf("error reloading, details: %v", err)
	// }
	return nil
}

func (cfg *fluentdConfig) write(infraCfg infraconfig.InfraLoggingConfig) (err error) {
	var w io.Writer

	w, err = os.Create(cfg.TmpConfigPath)
	if err != nil {
		return errors.Wrap(err, "fluentd create temp config file state error")
	}

	if _, err := os.Stat(cfg.TmpConfigPath); err != nil {
		return errors.Wrap(err, "fluentd temp config file state error")
	}

	var t *template.Template
	t, err = template.ParseFiles(cfg.TemplateConfigPath)
	if err != nil {
		return err
	}
	conf := make(map[string]interface{})
	conf["stores"] = infraCfg.Targets
	conf["sources"] = infraCfg.Sources
	err = t.Execute(w, conf)
	return err
}
