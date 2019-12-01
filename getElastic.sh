#!/bin/sh
docker pull docker.elastic.co/elasticsearch/elasticsearch:7.4.2
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data docker.elastic.co/elasticsearch/elasticsearch:7.4.2 &

sleep 10

docker exec -it `docker ps | awk ' /elasticsearch/ { print $1 }'`  bash -c 'yes | bin/elasticsearch-plugin install ingest-attachment'
docker commit `docker ps | awk ' /elasticsearch/ { print $1 }'` elastic-with-plugin