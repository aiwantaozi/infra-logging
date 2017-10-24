package config

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	logging "github.com/aiwantaozi/infra-logging-client/logging"
	loggingv1 "github.com/aiwantaozi/infra-logging-client/logging/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

const (
	SecretName       = "loggingsecret"
	AwsElasticsearch = "aws-elasticsearch"
	Elasticsearch    = "elasticsearch"
	Splunk           = "splunk"
	Kafka            = "kafka"
	Embedded         = "embedded"
)

var (
	k8sConfigPath    string
	SecretConfigDir  string
	TargetUIToFlentd = map[string]string{
		AwsElasticsearch: "aws-elasticsearch-service",
		Elasticsearch:    "elasticsearch",
		Splunk:           "splunk-http-eventcollector",
		Kafka:            "kafka_buffered",
		Embedded:         "elasticsearch",
	}
)

type InfraLoggingConfig struct {
	NamespaceTargets []Target `json:"namespaceTargets"`
	ClusterTarget    Target   `json:"clusterTarget"`
}

type Target struct {
	Secret           Secret `json:"secret"`
	FluentdMatchType string `json:"fluentdMatchType"`
	loggingv1.Target
	Namespace string `json:"namespace"`
}

type Secret struct {
	TargetType string                 `json:"type"`
	Label      string                 `json:"label"`
	Data       map[string]interface{} `json:"data"`
}

func Init(c *cli.Context) {
	k8sConfigPath = c.String("k8s-config-path")
}

func IsReachable() error {
	cfg, err := NewClientConfig()
	if cfg == nil || err != nil || cfg.Host == "" {
		logrus.Error("Could not communicate with k8s")
		return errors.Wrap(err, "could not reach k8s")
	}
	return nil
}

func NewClientConfig() (cfg *rest.Config, err error) {
	if k8sConfigPath != "" {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.ExplicitPath = k8sConfigPath
		overrides := &clientcmd.ConfigOverrides{}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	}
	return rest.InClusterConfig()
}

func GetLoggingConfig(namespace, name string) (*InfraLoggingConfig, error) {
	cfg, err := NewClientConfig()
	if err != nil {
		return nil, err
	}
	mclient, err := logging.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	obj, err := mclient.LoggingV1().Loggings(namespace).List(metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("enable", "true").String()})
	if err != nil {
		return nil, err
	}

	loggingList, ok := obj.(*loggingv1.LoggingList)
	if !ok {
		return nil, errors.New("could not convert obj to LoggingList")
	}
	var nsTgs []Target
	var infraCfg InfraLoggingConfig
	for _, v := range loggingList.Items {
		tg := Target{
			Namespace: v.Namespace,
		}
		if v.Target.TargetType == Embedded {
			v.Target.ESHost = Elasticsearch + "." + loggingv1.ClusterNamespace
		}
		tg.FluentdMatchType = TargetUIToFlentd[v.Target.TargetType]
		tg.Target = v.Target
		sec, err := kclient.CoreV1().Secrets(v.Namespace).Get(loggingv1.SecretName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "get secret fail")
		}
		var secData Secret
		err = json.Unmarshal(sec.Data[loggingv1.SecretName], &secData)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal secret fail")
		}
		tg.Secret = secData
		if tg.Namespace == loggingv1.ClusterNamespace {
			infraCfg.ClusterTarget = tg
		} else {
			nsTgs = append(nsTgs, tg)
		}
	}

	infraCfg.NamespaceTargets = nsTgs
	return &infraCfg, nil
}
