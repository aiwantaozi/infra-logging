package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	LoggingsKind          = "Logging"
	LoggingResourcePlural = "loggings"
	LoggingName           = "rancherlogging"
	GroupName             = "rancher.com"
	Namespace             = "cattle-system"
	Version               = "v1"
	SecretName            = "loggingsecret"
	SecretPath            = "/fluentd/etc/k8ssecret"
	ProviderName          = "fluentd"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LoggingCRDName     = LoggingResourcePlural + "." + GroupName
	LabelMaps          = map[string]string{
		"mylabel": "test",
	}
)

type LoggingSpec struct {
	Provider      string   `json:"provider"`
	LatestVersion string   `json:"latest_version"`
	Sources       []Source `json:"sources"`
	Targets       []Target `json:"targets"`
	Volumes       []struct {
		Name   string `json:"name"`
		Secret struct {
			SecretName string `json:"secretName"`
		} `json:"secret"`
	} `json:"volumes"`
}

type Source struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	InputPath   string `json:"input_path"`
	InputFormat string `json:"input_format"`
}

type Target struct {
	Environment              string `json:"environment"`
	OutputType               string `json:"output_type"`
	OutputHost               string `json:"output_host"`
	OutputPort               int    `json:"output_port"`
	OutputLogstashPrefix     string `json:"output_logstash_prefix"`
	OutputLogstashDateformat string `json:"output_logstash_dateformat"`
	OutputTagKey             string `json:"output_tag_key"` // (optional; default=fluentd)
}

type LoggingStatus struct {
	State string `json:"state"`
	Host  []struct {
		HostID         string `json:"host_id"`
		CurrentVersion string `json:"current_version"`
		Status         string `json:"status"`
	}
}

func (spec LoggingSpec) Validate() error {
	return nil
}

type Logging struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              LoggingSpec   `json:"spec"`
	Status            LoggingStatus `json:"status,omitempty"`
}

const (
	loggingStatePending   = "Pending"
	loggingStateProcessed = "Processed"
	loggingError          = "Error"
)

type LoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Logging `json:"items"`
}
