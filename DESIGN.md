# infra-logging

## Implementation goals

1. Allow Rancher operators to route logs to data stores.
  * a. A single data store for all logs in a cluster.
  * b. A default data store for all logs in a cluster AND multiple user-specified data stores for the different namespace.
  * c. A default data store for all logs in a cluster OR multiple user-specified data stores for the different namespace.
2. Rancher provides a data store for logging along with visualization tools.

## Basic Functionality
1. Cluster level log set and collect
2. Namespace level log set and collect, namespace target will not overwrite the cluster log.
3. Rancher service level log set and collect, As a user deploys a container/service a user can specify a log path, along with log types such as Apache, Nginx, Syslog, log4j
4. When the log configuration changes, the fluentd process should be reloaded, and use the latest config.
5. Support data store Elasticsearch, Splunk, Syslog and Kafka. 
6. Support service log format Apache, Nginx, Syslog, log4j.
7. Support deploy by the k8s yaml, use secret and deamonset.

## Implementation

### Watcher: watch CRD change, reload fluentd
1. Defining logging CustomerResourceDefine, cluster logging and namespace logging share the same CRD, but name and namespace are different.
2. Using the kubectl to trigger the change instead of UI.
3. Running fluentd. First time deploy, it will get the fluentd configuration from k8s and build the fluentd configure file.
4. Watching cluster level logging CRD change, namespace level logging CRD change. After deployed, watch the CRD change from k8s, include ClusterLogging, Logging.
5. Will use k8s CRD field "resourceVersion" to get the latest log version, generate the latest config, diff with current node config, if changed, will generate new configuration and reload fluentd. 
5. Collecting docker log, the config generate will use fluentd kubernete metadata plugin to collect docker log, and it will add more tag, like the namespace.
6. Sending log to Elasticsearch.
7. Support different namespace logging, user could configure different target in the different namespace, the namespace target could not overwrite the cluster target.
8. When Rancher Logging is enabled, we will disable logging drivers for containers and ignore them in the compose files.
9. When it reloads fail, will fail in the health check. K8s support health check for command, HTTP, TCP, also could config the Probes, like initialDelaySeconds, periodSeconds, timeoutSeconds...Need to decide health check strategy.
10. For some third-party resource need access credential and extra-label in fluentd, also need to be handler. For example:

```
<match es.**>
  type "aws-elasticsearch-service"
  type_name "access_log"
  logstash_format true
  include_tag_key true
  tag_key "@log_name"
  flush_interval 1s

   <endpoint>
        access_key_id "my_key"
        region us-east-1
        secret_access_key my_sec
        url https://search-michelia-2h5waj2csclbqdedtyftfh6zte.us-east-1.es.amazonaws.com
    </endpoint>
</match>
```

### k8s Controller: support UI operate k8s CRD.
1. Add cluster logging resource and namespace logging resource API operate.
2. Resource CRUD function.

### k8s Service log: support generate service log annotation, mount log file to host
1. Collecting service log. For service/container log, user can define the log path and the log format, support format include Apache, Nginx, Syslog, log4j. Add supported fluentd plugin in the docker image.
2. Need to update Rancher agent to create a volume mount the user log to host path /var/log/volumes/<namespace>/<stack>/<service>/<format>/*.log, <namespace> and so on will be replaced, including the information about namespace, stack, service, format. 
4. All the host have the same namespace configuration in fluentd, fluentd will get files in path  /var/log/volumes/<namespace>/**, if some namespace only in the fluentd config, but node don't have this namespace's pod, fluentd will get nothing and silence without any error.

### Fluentd Plugin
1. About the user-defined service, we need to write a fluentd plugin to collect the info from the path and add tag for namespace, stack, service, format
2. The fluentd plugin will handle to get the service log under the path /var/log/volumes/, and match the related log format, output target.

### Improve
1. Integrating with rancher, use the catalog to deploy the logging.
2. Supporting for more storage, include Elasticsearch, Splunk, Kafka.

<br>
The workflow is like this picture:
<br>

![sequence workflow](https://github.com/aiwantaozi/draw/blob/master/logging_sequence2.jpg)

Main conponment:
<br>

![conponent](https://github.com/aiwantaozi/draw/blob/master/logging_conponent.jpg)
<br>

Fluentd file:


```
# Cluster log
<source>
   @type  tail
   path  /var/log/containers/*.log
   pos_file  /fluentd/etc/fluentd-cluster-logging.pos
   time_format  %Y-%m-%dT%H:%M:%S
   tag  k8slogs.*
   format  json
   read_from_head  true
</source>
<source>
   @type  monitor_agent
   bind  0.0.0.0
   port  24220
</source>
<filter  k8slogs.**>
   type  kubernetes_metadata
   merge_json_log  true
   preserve_json_log  true
</filter>
<filter  k8slogs.**>
   @type  record_transformer
   enable_ruby  true
   <record>
      kubernetes_namespace_container_name $${record["kubernetes"]["namespace_name"]}.$${record["kubernetes"]["container_name"]}     
   </record>
</filter>
<match  k8slogs.**>
   @type  rewrite_tag_filter
   rewriterule1  kubernetes_namespace_container_name    ^(.+)$$  kubernetes.$$1
</match>

<filter  k8slogs.**>
   @type  record_transformer #Remove the unnecessary field as the information is already available in other fields.
   remove_keys  kubernetes_namespace_container_name
</filter>
<match  k8slogs.**>
  @copy
  <store> # cluster storage
    @type  elasticsearch
    host  es-client-lb.es-cluster.rancher.internal
    port  9200
    logstash_format  true
    logstash_prefix  rancher-k8s-testing
    logstash_dateformat  %Y%m%d             include_tag_key  true
    type_name  access_log
    tag_key  @log_name
    flush_interval  1s
  </store>

  <store> #namespace A storage
    @type  elasticsearch
    host  es-client-lb.es-cluster.rancher.internal
    port  9200
    logstash_format  true
    logstash_prefix  rancher-k8s-testing
    logstash_dateformat  %Y%m%d             include_tag_key  true
    type_name  access_log
    tag_key  @log_name
    flush_interval  1s
  </store>
</match>


# service log
<source>
  @type tail
  path /var/log/volumes/*/*/*/apache2/* # path is /var/log/volumes/<namespace>/<stack>/<service>/<type>/*.log
  pos_file /var/log/volumes/namespaceA/mystack/myservice/var/log/my.log.pos
  tag namespacelogging.*
  format apache2
</source>

<source>
  @type tail
  path /var/log/volumes/*/*/*/nginx/* # path is /var/log/volumes/<namespace>/<stack>/<service>/<type>/*.log
  pos_file /var/log/volumes/namespaceA/mystack/myservice/var/log/my.log.pos
  tag namespacelogging.*
  format nginx
</source>

<source>
  @type tail
  path /var/log/volumes/*/*/*/syslog/* # path is /var/log/volumes/<namespace>/<stack>/<service>/<type>/*.log
  pos_file /var/log/volumes/namespaceA/mystack/myservice/var/log/my.log.pos
  tag namespacelogging.*
  format syslog
</source>

<match  namespacelogging.var.log.volumes.namespaceA.**>
    @type  elasticsearch
    host  es-client-lb.es-cluster.rancher.internal
    port  9200
    logstash_format  true
    logstash_prefix  rancher-k8s-testing
    logstash_dateformat  %Y%m%d             include_tag_key  true
    type_name  access_log
    tag_key  @log_name
    flush_interval  1s
</match>

<match  namespacelogging.var.log.volumes.namespaceB.**>         
    @type  elasticsearch
    host  es-client-lb.es-cluster.rancher.internal            
    port  9200
    logstash_format  true
    logstash_prefix  fluentd-kubernetes-k8s
    logstash_dateformat  %Y%m%d
    include_tag_key  true
    type_name  access_log #default fluentd
    tag_key  @log_name
    flush_interval  1s
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

Fluentd stdout output example:
```
 @timestamp          October 23rd 2017, 16:01:10.000
 _id          AV9O690WEc-rbWmWKtyB
 _index          logstash-2017.10.23
 _score         - 
 _type          fluentd
 docker.container_id          649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a
 kubernetes.container_name          kubernetes-dashboard
 kubernetes.namespace_name          kube-system
 kubernetes.pod_name          kubernetes-dashboard-548151799-993ms
 log          Creating API server client for https://10.254.0.1:443
 stream          stdout
 tag          kubernetes.var.log.containers.kubernetes-dashboard-548151799-993ms_kube-system_kubernetes-dashboard-649c416c8f6c8d6aad18a7f6287eb17c94c222f855620c7981f8bdebcd0f407a.log
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


Fluentd Service log output:
JSON:
```
{
  "_index": "logstash-2010.08.23",
  "_type": "fluentd",
  "_id": "AV-VVCi3SqQCnf5RjWKO",
  "_version": 1,
  "_score": null,
  "_source": {
    "remote": "123.65.150.10",
    "host": "-",
    "user": "-",
    "method": "POST",
    "path": "/wordpress3/wp-admin/admin-ajax.php",
    "code": "200",
    "size": "2",
    "referer": "http://www.example.com/wordpress3/wp-admin/post-new.php",
    "agent": "Mozilla/5.0 (Macintosh; U; Intel Mac OS X 10_6_4; en-US) AppleWebKit/534.3 (KHTML, like Gecko) Chrome/6.0.472.25 Safari/534.3",
    "tag": "servicelog.var.log.volumes.ns-1.mystack.myservice.nginx.mylog.log",
    "namespace": "ns-1",
    "stack_name": "mystack",
    "service_name": "myservice",
    "log_format": "nginx",
    "log_type": "service_log",
    "@timestamp": "2010-08-23T11:50:59+08:00"
  },
  "fields": {
    "@timestamp": [
      1282535459000
    ]
  },
  "sort": [
    1282535459000
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
#### Target
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
### Namespace
#### Target
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
#### Input Source
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
kind: Logging
metadata:
  name: k8slogs
  namespace: cattle-system
target: 
  output_type: elasticsearch
  es_host: 192.168.33.176
  es_port: 9200
  output_logstash_prefix: "logstash"
  output_logstash_dateformat: '%Y-%m-%d %H:%M:%S'
  output_tag_key: "mytag3"
```

### Logging
```
apiVersion: "rancher.com/v1"
kind: Logging
metadata:
  name: namespacelogging-namespaceA
  namespace: namespaceA
target: 
   output_type: elasticsearch
   es_host: 192.168.33.176
   es_port: 9200
   output_logstash_prefix: "logstash"
   output_logstash_dateformat: '%Y-%m-%d %H:%M:%S'
   output_tag_key: "mytag3"
```
### Pod annotation
```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
    - name: client-container
      image: nginx
      volumeMounts:
        - name: hostdir
          mountPath: /var/log
          readOnly: false
  volumes:
  - name: hostdir
    hostPath:
      path: /var/log/volumes/namespaceA/mystack/myservice/apacheType/
      type: DirectoryOrCreate
```

## Different Target Support

### Splunk
* fluentd plugin: https://github.com/aiwantaozi/fluent-plugin-splunk-http-eventcollector.git
* config
```
<match kubernetes.**>
  @type splunk-http-eventcollector

  # server: Splunk server host and port
  # default: localhost:8088
  server 192.168.1.183:8088

  all_items true
  # protocol: Connect to Splunk server via 'http' or 'https'
  # default: https
  protocol http

  # verify: SSL server verification
  # default: true
  # verify false

  # token: the token issued
  token 94233fd4-58eb-45ff-a8b6-c67c49d0140c

  #
  # Event Parameters
  #

  # host: 'host' parameter passed to Splunk

  # index: 'index' parameter passed to Splunk (REST only)
  # default: <none>
  # index main

  # check_index: 'check-index' parameter passed to Splunk (REST only)
  # default: <none>
  # check_index false

  # host: 'source' parameter passed to Splunk
  # default: {TAG}
  #
  # "{TAG}" will be replaced by fluent tags at runtime
  # source {TAG}

  # sourcetype: 'sourcetype' parameter passed to Splunk
  # default: fluent
  sourcetype testsource

  #
  # Formatting Parameters
  #

  # time_format: the time format of each event
  # value: none, unixtime, localtime, or any time format string
  # default: localtime
  time_format localtime

  # format: the text format of each event
  # value: json, kvp, or text
  # default: json
  #
  # input = {"x":1, "y":"xyz", "message":"Hello, world!"}
  # 
  # 'json' is JSON encoding:
  #   {"x":1,"y":"xyz","message":"Hello, world!"}
  # 
  # 'kvp' is "key=value" pairs, which is automatically detected as fields by Splunk:
  #   x="1" y="xyz" message="Hello, world!"
  # 
  # 'text' outputs the value of "message" as is, with "key=value" pairs for others:
  #   [x="1" y="xyz"] Hello, world!
  format json

  #
  # Buffering Parameters
  #

  # Standard parameters for buffering.  See documentation for details:
  #   http://docs.fluentd.org/articles/buffer-plugin-overview
  buffer_type memory
  buffer_queue_limit 16

  # buffer_chunk_limit: The maxium size of POST data in a single API call.
  # 
  # This value should be reasonablly small since the current implementation
  # of out_splunk-http-eventcollector converts a chunk to POST data on memory before API calls.
  # The default value should be good enough.
  buffer_chunk_limit 8m

  # flush_interval: The interval of API requests.
  # 
  # Make sure that this value is sufficiently large to make successive API calls.
  # Note that a different 'source' creates a different API POST, each of which may
  # take two or more seconds.  If you include "{TAG}" in the source parameter and
  # this 'match' section recieves many tags, a single flush may take long time.
  # (Run fluentd with -v to see verbose logs.)
  flush_interval 1s
</match>

```

## Storage
The log data inside the embedded elasticsearch will be persisted.

### Concepts
* PersistentVolume (PV) is a piece of storage in the cluster that has been provisioned by an administrator. It is a resource in the cluster just like a node is a cluster resource. PVs are volume plugins like Volumes, but have a lifecycle independent of any individual pod that uses the PV. 
* PersistentVolumeClaim (PVC) is a request for storage by a user.
* StorageClass provides a way for administrators to describe the “classes” of storage they offer. It have four part, include:
  * Provisioner: Storage classes have a provisioner that determines what volume plugin is used for provisioning PVs.
  * Reclaim Policy
  * Mount Options
  * Parameter: Storage classes have parameters that describe volumes belonging to the storage class. 

### Storage we may choose
#### Local volume (external)
link:
[cn](https://github.com/feiskyer/kubernetes-handbook/blob/master/concepts/local-volume.md)
[en](https://github.com/kubernetes-incubator/external-storage/tree/master/local-volume)
#### HostPath
currently use

#### Rook
https://github.com/rook/rook
#### NFS

#### Glusterfs

#### Ceph RBD

#### Quobyte

#### Portworx Volume
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: portworx-io-priority-high
provisioner: kubernetes.io/portworx-volume
parameters:
  repl: "1"
  snap_interval:   "70"
  io_priority:  "high"
```

### ScaleIO
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: slow
provisioner: kubernetes.io/scaleio
parameters:
  gateway: https://192.168.99.200:443/api
  system: scaleio
  protectionDomain: pd0
  storagePool: sp1
  storageMode: ThinProvisioned
  secretRef: sio-secret
  readOnly: false
  fsType: xfs
```

#### StorageOS
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: fast
provisioner: kubernetes.io/storageos
parameters:
  pool: default
  description: Kubernetes volume
  fsType: ext4
  adminSecretNamespace: default
  adminSecretName: storageos-secret
```

#### Flocker 
Flocker is an open-source clustered container data volume manager. It provides management and orchestration of data volumes backed by a variety of storage backends.
## Refference
### Fluentd
* service format: https://docs.fluentd.org/v0.12/articles/common-log-formats