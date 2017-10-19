package logging

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"

	v1 "github.com/aiwantaozi/infra-logging/client/logging/v1"
)

var _ Interface = &Clientset{}

type Interface interface {
	LoggingV1() v1.LoggingV1Interface
}

type Clientset struct {
	*v1.LoggingV1Client
}

func (c *Clientset) LoggingV1() v1.LoggingV1Interface {
	if c == nil {
		return nil
	}
	return c.LoggingV1Client
}

func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error

	cs.LoggingV1Client, err = v1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	return &cs, nil
}
