package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
)

type LoggingV1Interface interface {
	RESTClient() rest.Interface
	LoggingsGetter
}

type LoggingV1Client struct {
	restClient    rest.Interface
	dynamicClient *dynamic.Client
}

func (c *LoggingV1Client) Loggings(namespace string) LoggingInterface {
	return newLoggings(c.restClient, c.dynamicClient, namespace)
}

func (c *LoggingV1Client) RESTClient() rest.Interface {
	return c.restClient
}

func NewForConfig(c *rest.Config) (*LoggingV1Client, error) {
	config := *c
	SetConfigDefaults(&config)
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewClient(&config)
	if err != nil {
		return nil, err
	}

	return &LoggingV1Client{client, dynamicClient}, nil
}

func SetConfigDefaults(config *rest.Config) {
	config.GroupVersion = &schema.GroupVersion{
		Group:   GroupName,
		Version: Version,
	}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	return
}
