FROM micheliac/infra-logging-base:v.0.0.1

COPY plugins /fluentd/etc/
COPY config /fluentd/etc/
COPY infra-logging /usr/bin/

RUN apt-get update \
     && apt-get install \
     -y --no-install-recommends \
     procps vim

CMD ["infra-logging"]

