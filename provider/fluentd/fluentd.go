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
	"k8s.io/client-go/pkg/api"

	loggingv1 "github.com/aiwantaozi/infra-logging-client/logging/v1"
	infraconfig "github.com/aiwantaozi/infra-logging/config"
	"github.com/aiwantaozi/infra-logging/provider"
)

const (
	ConfigFile   = "fluentd.conf"
	TmpFile      = "tmp.conf"
	PidFile      = "fluentd.pid"
	templateFile = "fluentd_template.conf"
	logPath      = "/fluentd/etc/log/fluentd.log"
)

var (
	fluentdProcess *exec.Cmd
	cfgPath        string
	cfgPathBak     string
	pidPath        string
	tmpPath        string
	templatePath   string
	fluentdTimeout = 1 * time.Minute
)

type Provider struct {
	cfg    fluentdConfig
	stopCh chan struct{}
	dryRun bool
}

//TODO change to lowercase
type fluentdConfig struct {
	Name      string
	StartCmd  string
	ConfigDir string
	PluginDir string
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
	logp.cfg.PluginDir = c.String("fluentd-plugin-dir")
	logp.dryRun = c.Bool("fluentd-dry-run")
	cfgPath = path.Join(logp.cfg.ConfigDir, ConfigFile)
	cfgPathBak = path.Join(logp.cfg.ConfigDir, ConfigFile+".bak")
	pidPath = path.Join(logp.cfg.ConfigDir, PidFile)
	tmpPath = path.Join(logp.cfg.ConfigDir, TmpFile)
	templatePath = path.Join(logp.cfg.ConfigDir, templateFile)
	logp.cfg.StartCmd = "fluentd " + "-c " + cfgPath + " -p " + logp.cfg.PluginDir + " -d " + pidPath + " --log " + logPath
}

func (logp *Provider) GetName() string {
	return "fluentd"
}

func (logp *Provider) Run() {
	if logp.dryRun {
		return
	}
	cfg, err := infraconfig.GetLoggingConfig(api.NamespaceAll, loggingv1.LoggingName)
	if err != nil {
		logrus.Errorf("fail get logging config, details: %s", err.Error())
		<-logp.stopCh
		return
	}
	if err = logp.cfg.write(cfg); err != nil {
		logrus.Errorf("fail write fluentd config, details: %s", err.Error())
		<-logp.stopCh
		return
	}

	if err := logp.StartFluentd(); err != nil {
		logrus.Errorf("fail start fluentd, details: %s", err.Error())
		<-logp.stopCh
		return
	}
	<-logp.stopCh
}

func (logp *Provider) Stop() error {
	logrus.Warnf("shutting down provider %s", logp.GetName())
	close(logp.stopCh)
	return nil
}

func (logp *Provider) StartFluentd() error {
	if logp.dryRun {
		return nil
	}
	cmd := exec.Command("sh", "-c", logp.cfg.StartCmd)
	logrus.Infof("fluentd start command: %s", logp.cfg.StartCmd)
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

func (logp *Provider) Reload() error {
	if logp.dryRun {
		return nil
	}
	pidFile, err := ioutil.ReadFile(pidPath)
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(pidFile)))
	if err != nil {
		return fmt.Errorf("fail parsing pid from %s: %s", pidFile, err)
	}

	if pid <= 0 {
		logrus.Warning("Fluentd not start yet, could not reload")
		return nil
	}
	if _, err := os.FindProcess(pid); err != nil {
		return fmt.Errorf("fail find process pid: %d, details: %v", pid, err)
	}

	if err = syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("fail reloading, details: %v", err)
	}
	return nil
}

func (logp *Provider) ApplyConfig(infraCfg *infraconfig.InfraLoggingConfig) error {
	err := logp.cfg.write(infraCfg)
	if err != nil {
		return err
	}
	err = logp.Reload()
	if err != nil {
		return err
	}
	return nil
}

func (cfg *fluentdConfig) write(infraCfg *infraconfig.InfraLoggingConfig) (err error) {
	var w io.Writer
	w, err = os.Create(tmpPath)
	if err != nil {
		return errors.Wrap(err, "fail create create fluentd temp config")
	}

	if _, err := os.Stat(tmpPath); err != nil {
		return errors.Wrap(err, "fail get created fluentd temp config file")
	}

	var t *template.Template
	t, err = template.ParseFiles(templatePath)
	if err != nil {
		return err
	}
	conf := make(map[string]interface{})
	conf["nsTargets"] = infraCfg.NamespaceTargets
	conf["clusterTarget"] = infraCfg.ClusterTarget
	err = t.Execute(w, conf)
	if err != nil {
		return err
	}

	// only change fluentd config when real change happen
	cfgEqual, err := isConfigEqual()
	if err != nil {
		return err
	}
	if cfgEqual {
		logrus.Info("config file not change, no need to reload")
		return nil
	}
	logrus.Info("config file changed, reloading")
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

func isConfigEqual() (bool, error) {
	f1, err := ioutil.ReadFile(tmpPath)

	if err != nil {
		return false, errors.Wrapf(err, "fail read file %s", tmpPath)
	}

	f2, err := ioutil.ReadFile(cfgPath)

	if err != nil {
		return false, errors.Wrapf(err, "fail read file %s", cfgPath)
	}
	return bytes.Equal(f1, f2), nil
}
