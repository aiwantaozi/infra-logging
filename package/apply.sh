#!/bin/bash

fluentd_base=/fluentd/etc/
fluentd_pid=${fluentd_base}fluentd.pid
sudo mkdir -p $fluentd_base
sudo chmod 777 $fluentd_base
touch $fluentd_pid
