package fluentd

import (
	"path"
	"time"

	"k8s.io/client-go/util/workqueue"

	"github.com/Sirupsen/logrus"
	"github.com/go-fsnotify/fsnotify"
	"github.com/pkg/errors"

	loggingv1 "github.com/aiwantaozi/infra-logging/client/logging/v1"
	infraConfig "github.com/aiwantaozi/infra-logging/config"
	"github.com/aiwantaozi/infra-logging/provider"
)

const (
	fileMonitorQueue = "file_monitor_queue"
)

type Controller struct {
	queue    workqueue.RateLimitingInterface
	provider provider.LogProvider
	stopCh   chan struct{}
}

func NewController(prd provider.LogProvider) *Controller {
	o := &Controller{
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fileMonitorQueue),
		provider: prd,
		stopCh:   make(chan struct{}),
	}
	return o
}

func (c *Controller) Run() error {
	defer c.queue.ShutDown()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "config file watch fail")
	}
	go c.worker()

	defer watcher.Close()
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				logrus.Debug("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					logrus.Debug("modified file:", event.Name)
					c.enqueue()
				}
			case err := <-watcher.Errors:
				logrus.Debug("error:", err)
			}
		}
	}()

	err = watcher.Add(path.Join(loggingv1.SecretPath, loggingv1.SecretName))
	if err != nil {
		logrus.Error(errors.Wrap(err, "Controller::Run, add watch file fail"))
	}

	<-c.stopCh
	return nil
}

func (c *Controller) Stop() {
	c.queue.ShutDown()
	close(c.stopCh)
}

func (c *Controller) enqueue() {
	key := c.keyFunc()
	logrus.Debug("controller enque, file change")
	c.queue.Add(key)
}

func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	logrus.Debug("controller processNextWorkItem, key:", key)
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	c.queue.AddRateLimited(key)
	return true
}

func (c *Controller) sync(key string) error {

	cfg, err := infraConfig.GetLoggingConfig(loggingv1.Namespace, loggingv1.LoggingName)
	if err != nil {
		return err
	}
	if err := c.provider.ApplyConfig(cfg); err != nil {
		return err
	}

	logrus.Infof("msg", "sync logging from file change, key:", key)

	return nil
}

func (c *Controller) keyFunc() string {
	return time.UTC.String()
}
