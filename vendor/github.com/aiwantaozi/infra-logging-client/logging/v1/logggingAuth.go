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

type LoggingAuthsGetter interface {
	LoggingAuths(namespace string) LoggingAuthInterface
}

var _ LoggingAuthInterface = &loggingAuths{}

type LoggingAuthInterface interface {
	Create(*LoggingAuth) (*LoggingAuth, error)
	Get(name string, opts metav1.GetOptions) (*LoggingAuth, error)
	Update(*LoggingAuth) (*LoggingAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type loggingAuths struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newLoggingAuths(r rest.Interface, c *dynamic.Client, namespace string) *loggingAuths {
	return &loggingAuths{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       LoggingAuthsKind,
				Name:       LoggingAuthResourcePlural,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (a *loggingAuths) Create(o *LoggingAuth) (*LoggingAuth, error) {
	ua, err := UnstructuredFromLoggingAuth(o)
	if err != nil {
		return nil, err
	}

	ua, err = a.client.Create(ua)
	if err != nil {
		return nil, err
	}

	return LoggingAuthFromUnstructured(ua)
}

func (a *loggingAuths) Get(name string, opts metav1.GetOptions) (*LoggingAuth, error) {
	obj, err := a.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return LoggingAuthFromUnstructured(obj)
}

func (a *loggingAuths) Update(o *LoggingAuth) (*LoggingAuth, error) {
	ua, err := UnstructuredFromLoggingAuth(o)
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

	return LoggingAuthFromUnstructured(ua)
}

func (a *loggingAuths) Delete(name string, options *metav1.DeleteOptions) error {
	return a.client.Delete(name, options)
}

func (a *loggingAuths) List(opts metav1.ListOptions) (runtime.Object, error) {
	req := a.restClient.Get().
		Namespace(a.ns).
		Resource(LoggingAuthResourcePlural).
		// VersionedParams(&options, api.ParameterCodec)
		FieldsSelectorParam(nil)

	var p LoggingAuthList
	b, err := req.DoRaw()
	if err != nil {
		fmt.Println("Got error ", err)
		return &p, err
	}

	return &p, json.Unmarshal(b, &p)
}

func (a *loggingAuths) Watch(opts metav1.ListOptions) (watch.Interface, error) {
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
	return watch.NewStreamWatcher(&loggingAuthDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil

}

func (a *loggingAuths) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return a.client.DeleteCollection(dopts, lopts)
}

// LoggingAuthFromUnstructured unmarshals an LoggingAuth object from dynamic client's unstructured
func LoggingAuthFromUnstructured(r *unstructured.Unstructured) (*LoggingAuth, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var a LoggingAuth
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	a.TypeMeta.Kind = LoggingAuthsKind
	a.TypeMeta.APIVersion = GroupName + "/" + Version
	return &a, nil
}

// UnstructuredFromLoggingAuth marshals an LoggingAuth object into dynamic client's unstructured
func UnstructuredFromLoggingAuth(a *LoggingAuth) (*unstructured.Unstructured, error) {
	a.TypeMeta.Kind = LoggingAuthsKind
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

type loggingAuthDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *loggingAuthDecoder) Close() {
	d.close()
}

func (d *loggingAuthDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object LoggingAuth
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
