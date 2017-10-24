FROM reg.cnrancher.com/logging/infra-logging-base:v.0.0.7
COPY plugins /fluentd/etc/plugins
COPY config /fluentd/etc/config
COPY infra-logging /usr/bin/

CMD ["infra-logging"]
