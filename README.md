infra-logging
========

A microservice that does micro things.

## Building

`make`


## Running

`./bin/infra-logging`

## Quick Deploy in Kubernetes

### Deployment main component
* kubectl run -f specs/usage/main, this command will deployment UI translator and fluentd controller daemonset, translator will work with UI part.

### Depployment thirdparty component
* Attention: This part is for deployment a Elasticsearch, Kibanna, and Splunk for you to test the target. if you have your own Elasticsearch and Splunk, you could ignore this part.
* kubectl run -f specs/usage/thirdparty_target

### Deployment option plugin
* Current we have a curator plugin, for scheduling delete Embedded Elasticsearch index, you could change the schedule job time at specs/usage/option/curator_cronjob.yaml

### Service log test
* You could deploy some service to test service log currently. kubectl run -f spec/usage/z_test

### Non UI Run
* These part could integrete with Kubernete without UI, without UI, you could use kubectl to create CRD and CRD object, related yaml are under specs/usage/z_none_ui, read the comment in the yaml file, config it with your own target.

## License
Copyright (c) 2014-2016 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
