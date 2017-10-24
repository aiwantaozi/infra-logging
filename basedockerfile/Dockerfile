FROM alpine:3.5
LABEL maintainer "Michelia <michelia.feng@gmail.com>"

ENV RUBY_GC_HEAP_OLDOBJECT_LIMIT_FACTOR 1.3
ENV DUMB_INIT_VERSION=1.2.0
ENV SU_EXEC_VERSION=0.2

RUN apk update \
 && apk upgrade \
 && apk add --no-cache \
        ca-certificates \
        ruby ruby-irb \
        su-exec==${SU_EXEC_VERSION}-r0 \
        dumb-init==${DUMB_INIT_VERSION}-r0 \
 && apk add --no-cache --virtual .build-deps \
        build-base \
        ruby-dev wget gnupg \
 && update-ca-certificates \
 && echo 'gem: --no-document' >> /etc/gemrc \
 && gem install oj -v 2.18.3 \
 && gem install json -v 2.1.0 \
 && gem install fluentd -v 0.14.23 \
 && gem install fluent-plugin-secure-forward \
 && gem install fluent-plugin-kubernetes_metadata_filter \
 && gem install fluent-plugin-elasticsearch \
 && gem install fluent-plugin-aws-elasticsearch-service \
 && gem install fluent-plugin-splunk-http-eventcollector \
 && gem install fluent-plugin-kafka \
 && gem install zookeeper \
 && gem install fluent-plugin-mongo \
 && gem install fluent-plugin-s3 \
 && gem install fluent-plugin-webhdfs \
 && apk del .build-deps \
 && rm -rf /var/cache/apk/* \
 && rm -rf /tmp/* /var/tmp/* /usr/lib/ruby/gems/*/cache/*.gem


RUN mkdir -p /fluentd/etc/log
RUN mkdir -p /fluentd/etc/buffer
# RUN chmod 777 /fluentd/etc/buffer

# 24224 for fluentd forward, 5140 for syslog forward
EXPOSE 24224 5140
