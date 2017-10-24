#!/bin/bash

 for line in $(curl -s http://localhost:9200/_cat/indices | cat |awk '{print $3}')
 do
    echo "$line"
    curl -X DELETE http://localhost:9200/"${line}"
 done