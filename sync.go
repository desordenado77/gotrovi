package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"path/filepath"

	"github.com/elastic/go-elasticsearch/esapi"
)

type folderOperation func(*Gotrovi, os.FileInfo, string)

func (gotrovi *Gotrovi) initializePipelineAttachment() {

	// configure Elastic
	body := "{ \"description\" : \"Extract attachment information\", \"processors\" : [ { \"attachment\" : { \"field\" : \"data\" }, \"remove\": { \"field\": \"data\" } } ] }"

	req := esapi.IngestPutPipelineRequest{DocumentID: "attachment", Body: strings.NewReader(body)}
	res, err := req.Do(context.Background(), gotrovi.es)
	if err != nil {
		Error.Println(err)
	}
	res.Body.Close()

	if res.IsError() {
		Error.Println("Unable to set pipeline attachement")
		Error.Println(err)
		os.Exit(1)
	}

}

func (gotrovi *Gotrovi) DeleteIndex() {
	Trace.Println("Deleting index" + GOTROVI_ES_INDEX)
	// Delete index to start from scratch
	req := esapi.IndicesDeleteRequest{Index: []string{GOTROVI_ES_INDEX}}
	res, err := req.Do(context.Background(), gotrovi.es)
	if err != nil {
		Error.Println(err)
	}
	res.Body.Close()
}

func count(g *Gotrovi, info os.FileInfo, p string) {
	g.total = g.total + 1
}

func putDoc(g *Gotrovi, r esapi.IndexRequest) (*http.Response, error) {

	// initialize http client
	client := &http.Client{}

	// marshal User to json
	//	json, err := json.Marshal(r.Body)
	//	if err != nil {
	//		panic(err)
	//	}

	// set the HTTP method, url, and request body
	req, err := http.NewRequest(http.MethodPut, "http://"+g.conf.ElasticSearch.Host+":"+strconv.Itoa(g.conf.ElasticSearch.Port)+"/"+GOTROVI_ES_INDEX+"/_doc/"+string(r.DocumentID)+"?pipeline=attachment", r.Body)
	if err != nil {
		Error.Println(err)
	}

	// set the request header Content-Type for json
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		Error.Println(err)
	}
	return resp, err
}

func deleteDoc(g *Gotrovi, r esapi.DeleteRequest) (*http.Response, error) {

	// initialize http client
	client := &http.Client{}

	// marshal User to json
	//	json, err := json.Marshal(r.Body)
	//	if err != nil {
	//		panic(err)
	//	}

	// set the HTTP method, url, and request body
	req, err := http.NewRequest(http.MethodDelete, "http://"+g.conf.ElasticSearch.Host+":"+strconv.Itoa(g.conf.ElasticSearch.Port)+"/"+GOTROVI_ES_INDEX+"/_doc/"+string(r.DocumentID), nil)
	if err != nil {
		Error.Println(err)
	}

	// set the request header Content-Type for json
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		Error.Println(err)
	}
	defer resp.Body.Close()
	return resp, err
}

func docExists(g *Gotrovi, r esapi.GetRequest) (exists bool) {

	// initialize http client
	client := &http.Client{}

	// set the HTTP method, url, and request body
	req, err := http.NewRequest(http.MethodGet, "http://"+g.conf.ElasticSearch.Host+":"+strconv.Itoa(g.conf.ElasticSearch.Port)+"/"+GOTROVI_ES_INDEX+"/_doc/"+string(r.DocumentID), nil)
	if err != nil {
		Error.Println(err)
		return false
	}

	// set the request header Content-Type for json
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if err != nil {
			Error.Println(err)
		}
		return false
	}
	resp.Body.Close()
	return true
}

func sync(g *Gotrovi, info os.FileInfo, p string) {
	Trace.Println("Sync File: " + p)

	var file FileDescriptionDoc

	file.Data = ""
	file.FileName = info.Name()
	file.Path = filepath.Dir(p)
	file.FullName = p
	file.Size = 0
	file.Extension = ""
	file.Hash = ""
	file.IsFolder = info.IsDir()
	file.Date = info.ModTime().String()
	file.Mode = info.Mode().String()

	if !info.IsDir() {
		f, err := os.Open(p)
		if err != nil {
			Error.Println(err)
			Error.Println()
		}
		defer f.Close()

		if _, err := io.Copy(g.hash, f); err != nil {
			Error.Println(err)
			Error.Println()
		}
		sum := g.hash.Sum(nil)

		f.Seek(0, io.SeekStart)

		// Read entire file into byte slice.
		reader := bufio.NewReader(f)
		content, _ := ioutil.ReadAll(reader)

		// Encode as base64.
		file.Data = base64.StdEncoding.EncodeToString(content)
		file.Size = info.Size()
		file.Extension = filepath.Ext(info.Name())
		file.Hash = fmt.Sprintf("%x", sum)
		g.hash.Reset()
		f.Close()
	}

	b, err := json.Marshal(file)
	if err != nil {
		Error.Println(err)
		Error.Println()
		return
	}

	//	fmt.Println(string(b))
	req := esapi.IndexRequest{
		Index:      GOTROVI_ES_INDEX,             // Index name
		Body:       strings.NewReader(string(b)), // Document body
		DocumentID: url.QueryEscape(p),           // url.QueryEscape(fmt.Sprintf("%x", g.hash.Sum([]byte(p)))), // strings.Replace(p, "/", "%2F", -1), // Document ID
		Pipeline:   "attachment",
		Refresh:    "true", // Refresh
	}

	// Cannot use the IndexRequest directly because esapi has issues handling forward slashes
	res, err := putDoc(g, req)
	if err != nil {
		Error.Println("Error getting response:", err)
		Error.Println()
		return
	}
	defer res.Body.Close()

	Trace.Println(res)
	if res.StatusCode != 201 && res.StatusCode != 200 {
		Error.Println("ES returned Error with file: ", p)
		body, _ := ioutil.ReadAll(res.Body)
		Error.Println("Error: ", string(body))
		Error.Println()
		return
	}

	g.writer.Clear()
	fmt.Fprintf(g.writer, "Synchronizing (%d/%d) files...\n", g.count, g.total)
	// write to terminal
	g.writer.Print()

	g.count = g.count + 1

}

func addMissing(g *Gotrovi, info os.FileInfo, p string) {
	Trace.Println("Checking if file exists in ES: " + p)

	req := esapi.GetRequest{
		Index:      GOTROVI_ES_INDEX,   // Index name
		DocumentID: url.QueryEscape(p), // url.QueryEscape(fmt.Sprintf("%x", g.hash.Sum([]byte(p)))), // strings.Replace(p, "/", "%2F", -1), // Document ID
	}
	if !docExists(g, req) {
		Info.Println("Adding file: ", p)
		Info.Println()
		sync(g, info, p)
		g.added = g.added + 1
	}

	g.writer.Clear()
	fmt.Fprintf(g.writer, "Checking for new files (%d/%d). %d files added...\n", g.count, g.total, g.added)
	// write to terminal
	g.writer.Print()

	g.count = g.count + 1

}

func (gotrovi *Gotrovi) PerformFolderOperation(id int, fo folderOperation) {
	f := gotrovi.conf.Index[id].Folder

	err := filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			Error.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return filepath.SkipDir
		}
		if info.IsDir() {
			for i := 0; i < len(gotrovi.conf.Index[id].Exclude); i++ {
				if path == gotrovi.conf.Index[id].Exclude[i] {
					Trace.Println("Skipping Folder (fullpath) " + path)
					return filepath.SkipDir
				}
			}
			for i := 0; i < len(gotrovi.conf.Exclude.Folder); i++ {
				if info.Name() == gotrovi.conf.Exclude.Folder[i] {
					Trace.Println("Skipping Folder (name) " + path)
					return filepath.SkipDir
				}
			}
		}
		// exclude extensions
		for i := 0; i < len(gotrovi.conf.Exclude.Extension); i++ {
			if filepath.Ext(path) == gotrovi.conf.Exclude.Extension[i] {
				Trace.Println("Skipping (ext) " + path)
				return nil
			}
		}

		if info.Size() > gotrovi.conf.Exclude.Size {
			Trace.Println("Skipping (size) " + path)
			return nil
		}

		//		fmt.Println(path)

		fo(gotrovi, info, path)

		return nil
	})
	if err != nil {
		Error.Println(err)
	}
}

func UpdateEntries(g *Gotrovi, total int, current int, e SearchHit, useHash bool, stringOption string, buf io.Writer) {
	// check if entry still exists and delete it from ES if not

	info, err := os.Stat(e.Source.FullName)
	if err != nil {
		Error.Println("Error getting file Stat " + e.Source.FullName)
		Error.Println("Error: ", err)
		Error.Println()
		return
	}
	if os.IsNotExist(err) {
		// file no longer present. Delete the document from ES
		Info.Println("Delete file from ES ", e.Source.FullName)
		Info.Println()

		req := esapi.DeleteRequest{
			Index:      GOTROVI_ES_INDEX, // Index name
			DocumentID: url.QueryEscape(e.Source.FullName),
		}
		// Cannot use the DeleteRequest directly because esapi has issues handling forward slashes
		res, err := deleteDoc(g, req)

		if err != nil || res.StatusCode != 200 {
			Error.Println("Error getting response:", err, res)
			Error.Println()
			return
		}
		res.Body.Close()
	} else {
		syncFile := false
		// Check if file has changed
		if useHash && !info.IsDir() {
			f, err := os.Open(e.Source.FullName)
			if err != nil {
				Error.Println(err)
			}
			defer f.Close()

			if _, err := io.Copy(g.hash, f); err != nil {
				Error.Println(err)
			}
			sum := g.hash.Sum(nil)

			f.Close()

			g.hash.Reset()

			if fmt.Sprintf("%x", sum) != e.Source.Hash {
				syncFile = true
			}

		}

		if !syncFile && info.ModTime().String() != e.Source.Date {
			syncFile = true
		}

		if syncFile {
			Info.Println("Resync file ", e.Source.FullName)
			Info.Println()
			sync(g, info, e.Source.FullName)

		}

	}

	g.writer.Clear()
	fmt.Fprintf(g.writer, "Updating (%d/%d) files...\n", total-current+1, total)
	// write to terminal
	g.writer.Print()

}

func (gotrovi *Gotrovi) SyncFolder(i int) {
	f := gotrovi.conf.Index[i].Folder
	Info.Println("- " + f)
	gotrovi.total = 0
	gotrovi.count = 0
	gotrovi.PerformFolderOperation(i, count)
	Info.Println("Found files: ", gotrovi.count)

	gotrovi.PerformFolderOperation(i, sync)

}

func (gotrovi *Gotrovi) SyncUpdate(useHash bool) {
	gotrovi.initializePipelineAttachment()

	Info.Println("Deleting missing docs")

	res, err := gotrovi.es.Search(
		gotrovi.es.Search.WithIndex(GOTROVI_ES_INDEX),
		//		gotrovi.es.Search.WithSort("timestamp:desc"),
		gotrovi.es.Search.WithSize(1),
		gotrovi.es.Search.WithContext(context.Background()),
	)
	if err != nil || res.IsError() {
		Trace.Println(err)
		return
	}
	defer res.Body.Close()

	//var buf bytes.Buffer

	Info.Println("Update existing entries")
	gotrovi.ES_Find("*", []string{}, useHash, "", false, UpdateEntries, os.Stdout)

}

func (gotrovi *Gotrovi) SyncAddMissing() {
	gotrovi.initializePipelineAttachment()

	Info.Println("Adding Missing files")

	for i := 0; i < len(gotrovi.conf.Index); i++ {
		f := gotrovi.conf.Index[i].Folder
		Info.Println("- " + f)
		gotrovi.total = 0
		gotrovi.count = 0
		gotrovi.added = 0
		gotrovi.PerformFolderOperation(i, count)
		Info.Println("Found files: ", gotrovi.count)

		gotrovi.PerformFolderOperation(i, addMissing)
	}
}

func (gotrovi *Gotrovi) SyncForced() {
	Info.Println("Performing Sync")

	res, err := gotrovi.es.Search(
		gotrovi.es.Search.WithIndex(GOTROVI_ES_INDEX),
		//		gotrovi.es.Search.WithSort("timestamp:desc"),
		gotrovi.es.Search.WithSize(1),
		gotrovi.es.Search.WithContext(context.Background()),
	)
	if err != nil {
		Error.Println(err)
	}
	defer res.Body.Close()

	if err == nil && !res.IsError() {
		gotrovi.DeleteIndex()
	}

	gotrovi.initializePipelineAttachment()

	for i := 0; i < len(gotrovi.conf.Index); i++ {
		gotrovi.SyncFolder(i)
	}
}
