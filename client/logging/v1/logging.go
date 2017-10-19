// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type LoggingsGetter interface {
	Loggings(namespace string) LoggingInterface
}

var _ LoggingInterface = &loggings{}

type LoggingInterface interface {
	Create(*Logging) (*Logging, error)
	Get(name string, opts metav1.GetOptions) (*Logging, error)
	Update(*Logging) (*Logging, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type loggings struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newLoggings(r rest.Interface, c *dynamic.Client, namespace string) *loggings {
	return &loggings{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       LoggingsKind,
				Name:       LoggingResourcePlural,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (a *loggings) Create(o *Logging) (*Logging, error) {
	ua, err := UnstructuredFromLogging(o)
	if err != nil {
		return nil, err
	}

	ua, err = a.client.Create(ua)
	if err != nil {
		return nil, err
	}

	return LoggingFromUnstructured(ua)
}

func (a *loggings) Get(name string, opts metav1.GetOptions) (*Logging, error) {
	obj, err := a.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return LoggingFromUnstructured(obj)
}

func (a *loggings) Update(o *Logging) (*Logging, error) {
	ua, err := UnstructuredFromLogging(o)
	if err != nil {
		return nil, err
	}

	cura, err := a.Get(o.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current version for update")
	}
	ua.SetResourceVersion(cura.ObjectMeta.ResourceVersion)

	ua, err = a.client.Update(ua)
	if err != nil {
		return nil, err
	}

	return LoggingFromUnstructured(ua)
}

func (a *loggings) Delete(name string, options *metav1.DeleteOptions) error {
	return a.client.Delete(name, options)
}

func (a *loggings) List(opts metav1.ListOptions) (runtime.Object, error) {
	req := a.restClient.Get().
		Namespace(a.ns).
		Resource(LoggingResourcePlural).
		// VersionedParams(&options, api.ParameterCodec)
		FieldsSelectorParam(nil)

	b, err := req.DoRaw()
	if err != nil {
		fmt.Println("Got error ", err)
		return nil, err
	}
	var p LoggingList
	return &p, json.Unmarshal(b, &p)
}

func (a *loggings) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := a.restClient.Get().
		Prefix("watch").
		Namespace(a.ns).
		Resource(LoggingResourcePlural).
		// VersionedParams(&options, api.ParameterCodec).
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&loggingDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil

}

func (a *loggings) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return a.client.DeleteCollection(dopts, lopts)
}

// LoggingFromUnstructured unmarshals an Logging object from dynamic client's unstructured
func LoggingFromUnstructured(r *unstructured.Unstructured) (*Logging, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var a Logging
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	a.TypeMeta.Kind = LoggingsKind
	a.TypeMeta.APIVersion = GroupName + "/" + Version
	return &a, nil
}

// UnstructuredFromLogging marshals an Logging object into dynamic client's unstructured
func UnstructuredFromLogging(a *Logging) (*unstructured.Unstructured, error) {
	a.TypeMeta.Kind = LoggingsKind
	a.TypeMeta.APIVersion = GroupName + "/" + Version
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var r unstructured.Unstructured
	if err := json.Unmarshal(b, &r.Object); err != nil {
		return nil, err
	}
	return &r, nil
}

type loggingDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *loggingDecoder) Close() {
	d.close()
}

func (d *loggingDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object Logging
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
