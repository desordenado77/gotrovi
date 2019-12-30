# Gotrovi

Trovi in speranto means find

This program indexes and performs searches in the files in your filesystem. It uses Elasticsearch as a search server, allowing the server to be on a different host. It uses the ingest attachment plugin which allows Elasticsearch to find text in many different types of files (pdf, word...).

It has 2 basic modes of operation: Sync and Find.

Sync will go through all the files in the filesystem and index them in Elasticsearch. There is a forced sync which will clean the Elastiseach gotrovi index and create new documnent entries for every file. There is also an update mode, in which the contents present in Elasticseach are compared with the filesystem and only updated if required. In the future, the plan is to use eBPF to track changed files and only sync in those.

Find will perform an elasticseach query, which uses lucene syntax, and display results on console.
