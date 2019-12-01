#!/bin/sh
#docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data docker.elastic.co/elasticsearch/elasticsearch:7.4.2
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data elastic-with-plugin
