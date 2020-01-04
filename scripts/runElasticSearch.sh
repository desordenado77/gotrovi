#!/bin/sh
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data docker.pkg.github.com/desordenado77/gotrovi-dockerfiles/gotrovi-es:7.4.2
