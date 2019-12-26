# Gotrovi

Trovi in speranto means find

This is a program to index and perform searches in your filesystem. It uses Elasticsearch as a search server, allowing the server to be on a different host. It uses the ingest attachment plugin which allows Elasticsearch to find text in many different types of files (pdf, word...).

It has 2 basic modes of operation: Sync and Find.

Sync will go through all the files in the filesystem and index them in Elasticsearch. Currently it performs a brute force sync by going through each and every file and sending its contents to Elasticsearch. The idea is to use eBPF to track changed files and only sync in those.

Find will perform an elasticseach query, which uses lucene syntax.
