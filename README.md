# Gotrovi

![GitHub Actions status](https://github.com/desordenado77/gotrovi/workflows/Go/badge.svg)

Trovi means find in esperanto

This program indexes and performs searches in the files in your filesystem. It uses Elasticsearch as a search server, allowing the server to be on a different host. It uses the ingest attachment plugin from ElasticSearch, which allows finding text in many different types of files (pdf, word...).

It has 2 basic modes of operation: Sync and Find.

Sync will go through all the files in the filesystem and index them in Elasticsearch. There is a forced sync which will clean the Elastiseach gotrovi index and create new documnent entries for every file. There is also an update mode, in which the contents present in Elasticseach are compared with the filesystem and only updated if required. In the future, the plan is to use eBPF to track changed files and only sync in those.

Find will perform an elasticseach query, which uses lucene syntax, and display results on console.

## How To Build

In order to build gotrovi you should do the following:

```sh
go get github.com/desordenado77/gotrovi
cd $GOPATH/src/github.com/desordenado77/gotrovi
dep ensure
go build
```

## How to Use

In order to use gotrovi, you need some prerequisites:

ElasticSearch: You need to have ElasticSearch running. ElasticSearch should have the attachment ingest plugin installed.

config.json: You need a gotrovi config file. Default location for the gotrovi config file is ".gotrovi/config.json". You may also redefine the location of the gotrovi config file by setting the environment variable GOTROVI_CONF to the folder where you have your config file. See bellow a sample config.json file.

```json
{
    "index": [
      {
        "folder": "/home/user",
        "exclude": [ "/home/user/.gotrovi" ]
      }
    ],
    "exclude": {
        "extension": [ ".o", ".bin", ".elf", ".zip", ".jpg", ".avi", ".mkv" ],
        "folder": [ ".git", ".svn" ],
        "size": 1000000
    },
    "hash": "md5",
    "elasticsearch": {
      "host": "localhost",
      "port": 9200
    }
}
```

"index" contains "folder", the folder to index, and "exclude" subfolders inside "folder" exclude from the indexing process.
"exclude" contains generic exclude rules for all files indexed. This includes extensions to exclude, folder names to exclude and max file to size to index.
"hash" is the hash method to use: md5, sha256, sha512.
"elasticsearch" contains the details of the ElasticSearch server to use, hostname and port number.

You can create a sample config.json and run an elasticsearch server container locally by calling gotrovi with the "-i" parameter (running the docker container will require having docker installed on the host):

```sh
gotrovi -i
```

Once this requisites are met, you need to index your selected folders from the filesystem. You can do that by doing:

```sh
gotrovi -s forced
```

Gotrovi will go through each and every file in the selected folders and index it in ElasticSearch for future searching. You need to manually resynch if there are changes in the filesystem. You can resynch by doing:

```sh
gotrovi -s update
```

Lastly, you may want to perform searches in your files, that is what gotrovi is for!!
For that purpose you use the "-f" parameter followed by a lucene query. More info on the Lucene query here: <https://lucene.apache.org/core/2_9_4/queryparsersyntax.html>

The format for the searches is the following:

```sh
gotrovi -f "query" folder
```

Where "query" is the lucene query to use for searching and folder is a optional parameter to restrict the search results to the mentioned folder and subfolders.

ElasticSearch documents have the following fields:

- filename
- fullpath
- path
- size
- extension
- hash
- isfolder
- date
- mode
- attachment.content
- attachment.content_type
- attachment.language

attachment.content is the field that will have the actual content of the file. The rest of the fields are hopefully self explanatory.

Here are some examples:

- Find files bigger than 10 bytes named test

```sh
gotrovi -f "size:>=10 AND filename:test"
```

- Find folders named test

```sh
gotrovi -f "filename:test AND isfolder:true"
```

- Find files containing test

```sh
gotrovi -f "attachment.content:test"
```

- Find files inside the current folder and subfolders containing test

```sh
gotrovi -f "attachment.content:test" ./
```

When performing a search there are some options to define how the results are reported:

- "-c": By using "-c" you can get for each search result the score reported.
- "-G": This will show a chunk of the document content where the search query is met, assuming the query is found on the document content.
- "-g value": Same as -G but it will highlihgt the word in value in the results.
