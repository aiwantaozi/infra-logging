package config

import (
	"encoding/json"
	"io/ioutil"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	logging "github.com/aiwantaozi/infra-logging/client/logging"
	loggingv1 "github.com/aiwantaozi/infra-logging/client/logging/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/glog"
)

var (
	kubeConfigPath string
)

type InfraLoggingConfig struct {
	Provider      string `json:"provider"`
	LatestVersion string `json:"latest_version"`
	Sources       []struct {
		Environment string `json:"environment"`
		InputPath   string `json:"input_path"`
		InputFormat string `json:"input_format"`
	} `json:"sources"`
	Targets []Target `json:"targets"`
}

type Target struct {
	Secret Secret `json:"secret"`
	Target loggingv1.Target
}

type Secret struct {
	Label string            `json:"label"`
	Data  map[string]string `json:"data"`
}

type SecretFileContent struct {
	Secrets []struct {
		Type       string            `json:"type"`
		Enviroment string            `json:"enviroment"`
		Label      string            `json:"label"`
		Data       map[string]string `json:"data"`
	} `json:"secrets"`
}

func NewClientConfig() (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	if kubeConfigPath != "" {
		rules.ExplicitPath = kubeConfigPath
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		glog.Fatalf("Couldn't get Kubernetes default config: %s", err)
		return nil, err
	}
	//return rest.InClusterConfig()
	return config, nil
}

func GetLoggingConfig(namespace, name string) (InfraLoggingConfig, error) {
	cfg, err := NewClientConfig()
	if err != nil {
		return InfraLoggingConfig{}, err
	}
	mclient, err := logging.NewForConfig(cfg)
	if err != nil {
		return InfraLoggingConfig{}, err
	}

	logging, err := mclient.LoggingV1().Loggings(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return InfraLoggingConfig{}, err
	}
	file, err := ioutil.ReadFile(loggingv1.SecretPath)
	if err != nil {
		return InfraLoggingConfig{}, err
	}

	var secFile SecretFileContent
	err = json.Unmarshal(file, &secFile)
	if err != nil {
		return InfraLoggingConfig{}, err
	}
	var tgs []Target
	for _, v := range logging.Spec.Targets {
		tg := Target{Target: v}
		for _, sec := range secFile.Secrets {
			if v.Environment == sec.Enviroment && v.OutputType == sec.Type {
				tg.Secret.Data = sec.Data
				tg.Secret.Label = sec.Label
			}
		}
		tgs = append(tgs, tg)
	}

	infraCfg := InfraLoggingConfig{
		Provider:      logging.Spec.Provider,
		LatestVersion: logging.Spec.LatestVersion,
		Sources:       logging.Spec.Sources,
		Targets:       tgs,
	}

	return infraCfg, nil
}
