# infra-logging

## Implementation goals

1. Allow Rancher operators to route logs to data stores.
  a. A single data store for all logs in a cluster.
  b. A default data store for all logs in a cluster AND multiple user specified data store.
  c. A default data store for all logs in a cluster OR multiple user specified data store.
2. Rancher provides a data store for logging along with visualization tools.

## Basic Functionality
1. Cluster level log set and collect
2. Environment level log set and collect, environment target will not over write the cluster log.
3. Rancher service level log set and collect, As a user deploys a container/service a user can specify a log path, along with log types such as Apache, Nginx, Syslog, log4j, etc for the actual log line.
4. When the log configuration is change, the fluentd process should be reload, and use the latest config.
5. Support data store elastic, splunk, syslog and kafka. 
6. Support service log format  Apache, Nginx, Syslog, log4j.
7. Support deploy by the rancher catalog.

## Implementation
1. Defining k8s CustomerResourceDefine, include fluentd configuration and the host environment information.
2. Using the kubectl to trigger the change instead of UI, because the UI part is not ready.
3. Running fluentd. First time deploy, it will get the fluentd configuration from k8s and build the fluentd configure file.
4. Watching crd change. After deployed, Watch the crd change from k8s, after config changed, generate new fluentd config and reload config
5. Collecting docker log, the config generate will use fluentd kubernete metadata plugin to collect docker log, and it will add more tag, like namespace.
6. Sending log to elastic-search.
7. Support different environment log, user could configure different target in different env, the env target could not overwrite the cluster env.
8. When Rancher Logging is enabled, we will disable logging drivers for containers and ignore them in the compose files.
9. Collecting service log. For service/container log, user can define the log path and the log format, support format include Apache, Nginx, Syslog, log4j.  Add support in fluentd config generator.
10. Need to update Rancher agent to create a volume mount the user log to path  /var/lib/docker/volumes/<name>/*/file, <name> will be replaced with the multiple directories, including the information about namespace, stack, service. 
11. About the user-defined service, we need to write a fluentd plugin to collect the info from the path and add tag.
12. Integrating with rancher, use catalog to deploy the logging.
13. Supporting for more storage, include elastic-search, splunk, kafka.

The workflow is like this picture:
![sequence picture](https://www.draw.io/?lightbox=1&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=logging_sequence.xml#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Faiwantaozi%2Fdraw%2Fmaster%2Flogging_sequence.xml)

Main conponment:
![conponent picture](https://www.draw.io/?lightbox=1&highlight=0000ff&edit=_blank&layers=1&nav=1&title=logging_conponent.xml#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Faiwantaozi%2Fdraw%2Fmaster%2Flogging_conponent.xml)

Fluentd file:

```
<source>
	@type   tail
	path   /var/log/containers/*.log
	pos_file   /fluentd/log/fluentd-kubernetes.pos
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

      #  Remove the unnecessary field as the information is already available on other fields.
      
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
created by k8s. For example:

```
/var/log/containers/kubernetes-dashboard-1319779976-66bh1_kube-system_kubernetes-dashboard-46c8a2a656e3b2516f06db72a8084b2a445e46adf2a8703d3c00de5cc6853965.log
/var/log/containers/nc-cc382a3f_default_dns-init-f388b5953fb3d92058ebd6ef7744f91a053af7c6d88019112b9ea893d1df5d7c.log
```

Log output example:
```
"10.42.147.50 - - [10/Oct/2017:07:33:26 +0000] \"GET / HTTP/1.1\" 304 0 \"-\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36\" \"192.168.16.1\"\r\n","stream":"stdout"
```

Fluentd output example:
```
2017-10-10 07:33:26 +0000 kubernetes.var.log.containers.nc-cc382a3f_default_nc-477a3d06-468e-435d-9729-e65c2e9568e6-dc3416d16f3ec64e453c0aa9494a7e7ef174a90c5a6f6648e9c8aa16548aac37.log: {"log":"10.42.147.50 - - [10/Oct/2017:07:33:26 +0000] \"GET / HTTP/1.1\" 304 0 \"-\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36\" \"192.168.16.1\"\r\n","stream":"stdout","docker":{"container_id":"dc3416d16f3ec64e453c0aa9494a7e7ef174a90c5a6f6648e9c8aa16548aac37"},"kubernetes":{"container_name":"nc-477a3d06-468e-435d-9729-e65c2e9568e6","namespace_name":"default","pod_name":"nc-cc382a3f","pod_id":"eb12e717-ad54-11e7-8a03-0242ac110002","labels":{"095e01a0":"0a4f233b","37588df4":"2cb09d4e","49fc8029":"1e734284","57e8c97c":"6670fdb8","714ff794":"f9f90eea","9a730256":"b76e98af","a089e9af":"b326b506","c4d44529":"7b13b53d","eaee61ea":"27b03b75","fce8991e":"b326b506","io_rancher_container_primary":"nc-477a3d06-468e-435d-9729-e65c2e9568e6","io_rancher_deployment_uuid":"cc382a3f-8b78-49c1-8a54-e205a84f10ee","io_rancher_revision":"4a40f7db4d42d8aa25fa3b266735db41"},"host":"node-1","master_url":"https://10.43.0.1:443/api"}}
```

## Attention
1. The FLuentd config secret must be created before they are consumed in pods as environment variables unless they are marked as optional. References to Secrets that do not exist will prevent the pod from starting. References via secretKeyRef to keys that do not exist in a named Secret will prevent the pod from starting. Secrets used to populate environment variables via envFrom that have keys that are considered invalid environment variable names will have those keys skipped, the pod will be allowed to start. 
2. Individual secrets are limited to 1MB in size. This is to discourage creation of very large secrets which would exhaust apiserver and kubelet memory. However, creation of many smaller secrets could also exhaust memory. More comprehensive limits on memory usage due to secrets is a planned feature.
3. For these reasons watch and list requests for secrets within a namespace are extremely powerful capabilities and should be avoided, since listing secrets allows the clients to inspect the values if all secrets are in that namespace. The ability to watch and list all secrets in a cluster should be reserved for only the most privileged, system-level components.
4. For improved performance over a looping get, clients can design resources that reference a secret then watch the resource, re-requesting the secret when the reference changes.
5. A secret is only sent to a node if a pod on that node requires it. It is not written to disk. It is stored in a tmpfs. It is deleted once the pod that depends on it is deleted.

## Filed can config
Cluster:
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
Enviroment:
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
Service:
* Output: Target type
* Output: Host
* Output: Port
* Input: Prefix
* Input: DateFormat
* Input: Tag
* Input: LogPath
* Input: Format


## Refference
### Fluentd
* service format: https://docs.fluentd.org/v0.12/articles/common-log-formats