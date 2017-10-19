package fluentd

import (
	"fmt"
	"log"
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

func NewController() *Controller {
	o := &Controller{
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fileMonitorQueue),
		provider: provider.GetProvider("fluentd"),
		stopCh:   make(chan struct{}),
	}
	return o
}

func (c *Controller) Run() error {
	defer c.queue.ShutDown()
	logrus.Debugf("Debug: Controller Run")
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
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
					c.enqueue()
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(loggingv1.SecretPath)
	if err != nil {
		log.Fatal(err)
	}

	<-c.stopCh
	return nil
}

func (c *Controller) Stop() {
	c.queue.ShutDown()
	close(c.stopCh)
}

// enqueue adds a key to the queue. If obj is a key already it gets added directly.
// Otherwise, the key is extracted via keyFunc.
func (c *Controller) enqueue() {
	key := c.keyFunc()
	fmt.Println("controller enque")
	c.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	fmt.Println("controller processNextWorkItem, key:", key)
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

	logrus.Info("msg", "sync logging from file change, key:", key)

	return nil
}

func (c *Controller) keyFunc() string {
	return time.UTC.String()
}
