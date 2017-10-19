package fluentd

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	logging "github.com/aiwantaozi/infra-logging/client/logging"
	loggingv1 "github.com/aiwantaozi/infra-logging/client/logging/v1"
	infraConfig "github.com/aiwantaozi/infra-logging/config"
	provider "github.com/aiwantaozi/infra-logging/provider"
)

const (
	resyncPeriod    = 5 * time.Minute
	k8sMonitorQueue = "k8s_monitor_queue"
)

type Operator struct {
	kclient   kubernetes.Interface
	mclient   logging.Interface
	crdclient apiextensionsclient.Interface

	loggingInf cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface

	config Config

	provider provider.LogProvider

	stopCh chan struct{}
}

type Config struct {
	Namespace string
	CrdGroup  string
	LabelsMap map[string]string
}

// NewOperator a new controller.
func NewOperator() (*Operator, error) {
	cfg, err := infraConfig.NewClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "instantiating cluster config failed")
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client failed")
	}

	mclient, err := logging.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating loggingv1 client failed")
	}

	crdclient, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating apiextensions client failed")
	}

	o := &Operator{
		kclient:   client,
		mclient:   mclient,
		crdclient: crdclient,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), k8sMonitorQueue),
		config: Config{
			Namespace: loggingv1.Namespace,
			CrdGroup:  loggingv1.GroupName,
			LabelsMap: loggingv1.LabelMaps,
		},
		provider: provider.GetProvider("fluentd"),
	}
	o.loggingInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  o.mclient.LoggingV1().Loggings(o.config.Namespace).List,
			WatchFunc: o.mclient.LoggingV1().Loggings(o.config.Namespace).Watch,
		},
		&loggingv1.Logging{}, resyncPeriod, cache.Indexers{},
	)

	o.loggingInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleLoggingAdd,
		DeleteFunc: o.handleLoggingDelete,
		UpdateFunc: o.handleLoggingUpdate,
	})

	return o, nil
}

// Run the controller.
func (c *Operator) Run() error {
	errChan := make(chan error)
	go func() {
		v, err := c.kclient.Discovery().ServerVersion()
		if err != nil {
			errChan <- errors.Wrap(err, "communicating with server failed")
			return
		}
		logrus.Infof("msg", "connection established", "cluster-version", v)

		errChan <- nil
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		logrus.Infof("msg", "CRD API endpoints ready")
	}
	go c.provider.Run()
	go c.worker()

	go c.loggingInf.Run(c.stopCh)

	<-c.stopCh
	return nil
}

func (c *Operator) Stop() {
	c.queue.ShutDown()
	close(c.stopCh)
}

func (c *Operator) keyFunc(obj interface{}) (string, bool) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logrus.Infof("msg", "creating key failed", "err", err)
		return k, false
	}
	return k, true
}

func (c *Operator) getObject(obj interface{}) (metav1.Object, bool) {
	ts, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = ts.Obj
	}

	o, err := meta.Accessor(obj)
	if err != nil {
		logrus.Infof("msg", "get object failed", "err", err)
		return nil, false
	}
	return o, true
}

// enqueue adds a key to the queue. If obj is a key already it gets added directly.
// Otherwise, the key is extracted via keyFunc.
func (c *Operator) enqueue(obj interface{}) {
	if obj == nil {
		return
	}

	key, ok := obj.(string)
	if !ok {
		key, ok = c.keyFunc(obj)
		if !ok {
			return
		}
	}

	c.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Operator) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Operator) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(errors.Wrap(err, fmt.Sprintf("Sync %q failed", key)))
	c.queue.AddRateLimited(key)

	return true
}

func (c *Operator) handleLoggingAdd(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	logrus.Infof("msg", "Logging added", "key", key)
	c.enqueue(key)
}

func (c *Operator) handleLoggingDelete(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	logrus.Infof("msg", "Logging deleted", "key", key)
	c.enqueue(key)
}

func (c *Operator) handleLoggingUpdate(old, cur interface{}) {
	key, ok := c.keyFunc(cur)
	if !ok {
		return
	}

	logrus.Infof("msg", "Logging updated", "key", key)
	c.enqueue(key)
}

func (c *Operator) sync(key string) error {
	obj, exists, err := c.loggingInf.GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		return c.destroyLogging(key)
	}

	am := obj.(*loggingv1.Logging)

	cfg, err := infraConfig.GetLoggingConfig(c.config.Namespace, loggingv1.LoggingName)
	if err != nil {
		return err
	}
	if err := c.provider.ApplyConfig(cfg); err != nil {
		return err
	}

	logrus.Infof("msg", "sync logging", "key", key, "name", am.ObjectMeta.Name)

	return nil
}

func (c *Operator) destroyLogging(key string) error {
	_, exists, err := c.loggingInf.GetStore().GetByKey(key)
	if err != nil {
		return errors.Wrap(err, "retrieving logging from cache failed")
	}
	if !exists {
		return nil
	}

	return nil
}
