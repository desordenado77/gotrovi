How to install ElasticSearch and the necessary plugins
------------------------------------------------------

Clone the gotrovi-dockerfiles repository:

git clone https://github.com/desordenado77/gotrovi-dockerfiles.git

Build the elasticsearch docker:

cd gotrovi-dockerfiles/elasticsearch
docker build ./ -t gotrovi-es:latest -t gotrovi-es:7.4.2

Run the container:

docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data gotrovi-es:latest

Old instructions:

Get docker container with elasticsearch by doing:

docker pull docker.elastic.co/elasticsearch/elasticsearch:7.4.2

Run the container (mapping the folder $PWD/es to /usr/share/elasticsearch/data):

docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data docker.elastic.co/elasticsearch/elasticsearch:7.4.2

On another terminal figure out the container id by doing:

docker ps

Run bash on the running container:

docker exec -it CONTAINER_ID bash

Install the ingest-attachment plugin:

bin/elasticsearch-plugin install ingest-attachment

Exit the container terminal and commit the changes:

docker commit CONTAINER_ID elastic-with-plugin

to run the newly modified container you need to do:

docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" -v ${PWD}/es:/usr/share/elasticsearch/data elastic-with-plugin





Running Kibana
--------------

Get Kibana:

docker pull kibana:7.4.2

Run Kibana (need to link it with the elasticsearch container):

docker run --link `docker ps | awk ' /elastic-with-plugin/ { print $1 }'`:elasticsearch -p 5601:5601 kibana:7.4.2




Info comes from:
https://www.techrepublic.com/article/how-to-commit-changes-to-a-docker-image/
https://www.elastic.co/guide/en/elasticsearch/plugins/5.2/ingest-attachment.html
