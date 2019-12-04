package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"path/filepath"

	"github.com/elastic/go-elasticsearch/esapi"
)

type folderOperation func(*Gotrovi, os.FileInfo, string)

func (gotrovi *Gotrovi) Sync() {
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
		Trace.Println("Deleting index" + GOTROVI_ES_INDEX)
		// Delete index to start from scratch
		req := esapi.IndicesDeleteRequest{Index: []string{GOTROVI_ES_INDEX}}
		res, err := req.Do(context.Background(), gotrovi.es)
		if err != nil {
			Error.Println(err)
		}
		defer res.Body.Close()
	}

	for i := 0; i < len(gotrovi.conf.Index); i++ {
		gotrovi.SyncFolder(i)
	}
}

func (gotrovi *Gotrovi) SyncFolder(i int) {
	f := gotrovi.conf.Index[i].Folder
	Info.Println("- " + f)
	gotrovi.count = 0
	gotrovi.PerformFolderOperation(i, count)
	Info.Println("Found files: ", gotrovi.count)

	switch gotrovi.conf.Hash {
	case "md5":
		gotrovi.hash = md5.New()
	case "sha256":
		gotrovi.hash = sha256.New()
	case "sha512":
		gotrovi.hash = sha512.New()
	default:
		gotrovi.hash = md5.New()

	}

	gotrovi.PerformFolderOperation(i, sync)

}

func count(g *Gotrovi, info os.FileInfo, p string) {
	g.count = g.count + 1
}

/*
type FileDescriptionDoc struct {
	FileName  string `json:"filename"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	Hash      string `json:"hash"`
	Data      string `json:"data"`
	isFolder  bool   `json:"isfolder"`
}
*/
func sync(g *Gotrovi, info os.FileInfo, p string) {
	Trace.Println("Sync File: " + p)

	var file FileDescriptionDoc

	file.Data = ""
	file.FileName = info.Name()
	file.Path = filepath.Dir(p)
	file.Size = 0
	file.Extension = ""
	file.Hash = ""
	file.IsFolder = info.IsDir()
	file.Date = info.ModTime().String()

	if !info.IsDir() {
		f, err := os.Open(p)
		if err != nil {
			Error.Println(err)
		}
		defer f.Close()

		if _, err := io.Copy(g.hash, f); err != nil {
			Error.Println(err)
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
	}

	b, err := json.Marshal(file)
	if err != nil {
		Error.Println(err)
		return
	}

	req := esapi.IndexRequest{
		Index:      GOTROVI_ES_INDEX,                                          // Index name
		Body:       strings.NewReader(string(b)),                              // Document body
		DocumentID: url.QueryEscape(fmt.Sprintf("%x", g.hash.Sum([]byte(p)))), // strings.Replace(p, "/", "%2F", -1), // Document ID
		Pipeline:   "attachment",
		Refresh:    "true", // Refresh
	}

	Trace.Println(req)
	res, err := req.Do(context.Background(), g.es)
	if err != nil {
		Error.Println("Error getting response:", err)
		return
	}
	defer res.Body.Close()

	Trace.Println(res)
	if res.IsError() {
		Error.Println("ES returned Error", res)
		return
	}
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
