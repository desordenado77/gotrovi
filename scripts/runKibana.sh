#!/bin/sh
docker run --link `docker ps | awk ' /gotrovi-es/ { print $1 }'`:elasticsearch -p 5601:5601 kibana:7.4.2
