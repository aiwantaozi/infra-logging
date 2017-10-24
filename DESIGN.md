# infra-logging

## Implementation goals

1. Allow Rancher operators to route logs to data stores.
  a. A single data store for all logs in a cluster.
  b. A default data store for all logs in a cluster AND multiple user-specified data stores for the different namespace.
  c. A default data store for all logs in a cluster OR multiple user-specified data stores for the different namespace.
2. Rancher provides a data store for logging along with visualization tools.

## Basic Functionality
1. Cluster level log set and collect
2. Namespace level log set and collect, namespace target will not overwrite the cluster log.
3. Rancher service level log set and collect, As a user deploys a container/service a user can specify a log path, along with log types such as Apache, Nginx, Syslog, log4j, etc for the actual log line.
4. When the log configuration changes, the fluentd process should be reloaded, and use the latest config.
5. Support data store elastic, Splunk, Syslog and Kafka. 
6. Support service log format Apache, Nginx, Syslog, log4j.
7. Support deploy by the k8s yaml.

## Implementation

### Watcher: watch crd change, reload fluentd
1. Defining 2 k8s CustomerResourceDefine, cluster level logging config crd, namespace level logging config crd.
2. Using the kubectl to trigger the change instead of UI.
3. Running fluentd. First time deploy, it will get the fluentd configuration from k8s and build the fluentd configure file.
4. Watching cluster level logging crd change, namespace level logging crd change. After deployed, Watch the crd change from k8s, after config changed, generate new fluentd config and reload config.
5. Will use k8s crd field"resourceVersion" to get the latest log version, diff with current config with the current node to check whether the config is changed.
5. Collecting docker log, the config generate will use fluentd kubernete metadata plugin to collect docker log, and it will add more tag, like the namespace.
6. Sending log to elastic-search.
7. Support different namespace log, user could configure different target in different env, the env target could not overwrite the cluster env.
8. When Rancher Logging is enabled, we will disable logging drivers for containers and ignore them in the compose files.
9. When it reloads fail will fail in the health check.

### k8s Controller: support UI operate k8s crd.
1. Add cluster logging resource and namespace logging resource API operate.
2. Resource CRUD function.

### k8s Service log: support generate service log annotation, volume mount and so on
1. Collecting service log. For service/container log, user can define the log path and the log format, support format include Apache, Nginx, Syslog, log4j.  Add support in fluentd config generator.
2. Need to update Rancher agent to create a volume mount the user log to path  /var/lib/docker/volumes/<name>/*/file, <name> will be replaced with the multiple directories, including the information about namespace, stack, service. 
3. About the user-defined service, we need to write a fluentd plugin to collect the info from the path and add tag.
4. Integrating with rancher, use catalog to deploy the logging.
5. Supporting for more storage, include elastic-search, Splunk, Kafka.

<br>
The workflow is like this picture:
<br>

![sequence workflow](https://github.com/aiwantaozi/draw/blob/master/logging_sequence.jpg)

Main conponment:
<br>

![conponent](https://github.com/aiwantaozi/draw/blob/master/logging_conponent.jpg)
<br>

Fluentd file:

```
<source>
    @type   tail
    path   /var/log/containers/*.log
    pos_file   /fluentd/etc/fluentd-kubernetes.pos
    time_format   %Y-%m-%dT%H:%M:%S
    tag   kubernetes.*
    format   json
    read_from_head   true
</source>
<source>
    @type   monitor_agent
    bind   0.0.0.0
    port   24220
</source>
<filter   kubernetes.**>
    type   kubernetes_metadata
    merge_json_log   true
    preserve_json_log   true
</filter>
<filter   kubernetes.**>
    @type   record_transformer
    enable_ruby   true
    <record>
        kubernetes_namespace_container_name $${record["kubernetes"]["namespace_name"]}.$${record["kubernetes"]["container_name"]}       
    </record>
</filter>
#   retag   based   on   the   container   name   of   the   log   message
<match   kubernetes.**>
    @type   rewrite_tag_filter
    rewriterule1   kubernetes_namespace_container_name      ^(.+)$$   kubernetes.$$1
</match>

      #  Remove the unnecessary field as the information is already available in other fields.
      
<filter   kubernetes.**>
    @type   record_transformer
    remove_keys   kubernetes_namespace_container_name
</filter>
<match   kubernetes.testing.**>
    @type   copy
    <store>
        @type   elasticsearch
        host   es-client-lb.es-cluster.rancher.internal
        port   9200
        logstash_format   true
        logstash_prefix   rancher-k8s-testing
        logstash_dateformat   %Y%m%d                   include_tag_key   true
        type_name   access_log
        tag_key   @log_name
        flush_interval   1s
    </store>
</match>
<match   kubernetes.**>             
    @type   copy                 
    <store>
        @type   elasticsearch
        host   es-client-lb.es-cluster.rancher.internal                 
        port   9200
        logstash_format   true
        logstash_prefix   fluentd-kubernetes-k8s
        logstash_dateformat   %Y%m%d
        include_tag_key   true
        type_name   access_log
        tag_key   @log_name
        flush_interval   1s
    </store> 
</match>

```

Will watch the path /var/log/containers/*.log, the symbol link sequece is /var/log/containers/*.log --> /var/log/pods/*.log --> /var/lib/docker/container/*/*-json.logs. The symbol link /var/log/containers/*.log is 
created by k8s. 

Log file name example:

```
kubernetes-dashboard-548151799-993ms_kube-system_kubernetes-dashboard-649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a.log
mydep-4169708966-cl4m3_default_mynginx2-ea6b0a7a8caaac2b64baec2f42acfe2edc51f433b2297b7cc81f72d9e4abe536.log
```

Format: podname_namespace_containername-containerid

Log output example:
```
"10.42.147.50 - - [10/Oct/2017:07:33:26 +0000] \"GET / HTTP/1.1\" 304 0 \"-\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36\" \"192.168.16.1\"\r\n","stream":"stdout"
```

Fluentd output example:
```
 @timestamp               October 23rd 2017, 16:01:10.000
 _id              AV9O690WEc-rbWmWKtyB
 _index              logstash-2017.10.23
 _score             - 
 _type              fluentd
 docker.container_id               649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a
 kubernetes.container_name               kubernetes-dashboard
 kubernetes.namespace_name               kube-system
 kubernetes.pod_name               kubernetes-dashboard-548151799-993ms
 log               Creating API server client for https://10.254.0.1:443
 stream               stdout
 tag               kubernetes.var.log.containers.kubernetes-dashboard-548151799-993ms_kube-system_kubernetes-dashboard-649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a.log
```

JSON:
```
{
  "_index": "logstash-2017.10.23",
  "_type": "fluentd",
  "_id": "AV9O690WEc-rbWmWKtyB",
  "_version": 1,
  "_score": null,
  "_source": {
    "log": "Creating API server client for https://10.254.0.1:443\n",
    "stream": "stdout",
    "docker": {
      "container_id": "649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a"
    },
    "kubernetes": {
      "container_name": "kubernetes-dashboard",
      "namespace_name": "kube-system",
      "pod_name": "kubernetes-dashboard-548151799-993ms"
    },
    "@timestamp": "2017-10-23T23:01:10+00:00",
    "tag": "kubernetes.var.log.containers.kubernetes-dashboard-548151799-993ms_kube-system_kubernetes-dashboard-649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a.log"
  },
  "fields": {
    "@timestamp": [
      1508799670000
    ]
  },
  "sort": [
    1508799670000
  ]
}
```

## Attention
1. The Fluentd config secret must be created before they are consumed in pods as environment variables unless they are marked as optional. References to Secrets that do not exist will prevent the pod from starting. References via secretKeyRef to keys that do not exist in a named Secret will prevent the pod from starting. Secrets used to populate environment variables via envFrom that have keys that are considered invalid environment variable names will have those keys skipped, the pod will be allowed to start. 
2. Individual secrets are limited to 1MB in size. This is to discourage creation of very large secrets which would exhaust apiserver and kubelet memory. However, creation of many smaller secrets could also exhaust memory. More comprehensive limits on memory usage due to secrets is a planned feature.
3. For these reasons watch and list requests for secrets within a namespace are extremely powerful capabilities and should be avoided, since listing secrets allows the clients to inspect the values if all secrets are in that namespace. The ability to watch and list all secrets in a cluster should be reserved for only the most privileged, system-level components.
4. For improved performance over a looping get, clients can design resources that reference a secret then watch the resource, re-requesting the secret when the reference changes.
5. A secret is only sent to a node if a pod on that node requires it. It is not written to disk. It is stored in a tmpfs. It is deleted once the pod that depends on it is deleted.

## Filed can config
### Cluster
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
### Namespace
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
Service Related:
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
* Input: LogPath
* Input: Format

## CRD yaml

### Cluster Logging

```
apiVersion: "rancher.com/v1"
kind: ClusterLogging
metadata:
  name: rancherlogging
  namespace: cattle-system
target: 
  output_type: elasticsearch
  output_host: 192.168.33.176
  output_port: 9200
  output_logstash_prefix: "logstash"
  output_logstash_dateformat: '%Y-%m-%d %H:%M:%S'
  output_tag_key: "mytag3"
```

### Logging
```
apiVersion: "rancher.com/v1"
kind: Logging
metadata:
  name: rancherlogging
  namespace: cattle-system
sources:
  - name: "stack_service"
    input_path: /fluentd/etc/my.log
    input_format: apache2
target: 
    output_type: elasticsearch
    output_host: 192.168.33.176
    output_port: 9200
    output_logstash_prefix: "logstash"
    output_logstash_dateformat: '%Y-%m-%d %H:%M:%S'
    output_tag_key: "mytag3"
```
### Pod annotation


## Refference
### Fluentd
* service format: https://docs.fluentd.org/v0.12/articles/common-log-formats