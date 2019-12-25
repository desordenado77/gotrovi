package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/gookit/color"
)

type Source struct {
	FileName  string `json:"filename"`
	FullName  string `json:"fullpath"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	Hash      string `json:"hash"`
	IsFolder  bool   `json:"isfolder"`
	Date      string `json:"date"`
	Mode      string `json:"mode"`
}

type SearchHit struct {
	Score  float64 `json:"_score"`
	Source Source  `json:"_source"`
}

type TotalHits struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}

type SearchHits struct {
	Hits  []SearchHit `json:"hits"`
	Total TotalHits   `json:"total"`
}

type SearchResult struct {
	ScrollId string     `json:"_scroll_id"`
	Hits     SearchHits `json:"hits"`
}

func PrintEntry(s Source, score float64, buf *bytes.Buffer) {
	colorfn := color.FgWhite.Render
	if s.IsFolder {
		colorfn = color.FgBlue.Render
	} else {
		if strings.Contains(s.Mode, "x") {
			colorfn = color.FgGreen.Render
		}
	}

	fmt.Fprintf(buf, "%s\t%g\n", colorfn(s.FullName), score)
}

func (gotrovi *Gotrovi) Find(name string, p string) {
	query := name

	fmt.Println(p)
	if p != "" {
		/*		var err error
				dir, err = os.Getwd()
				if err != nil {
					Error.Println(err)
					os.Exit(1)
				}
		*/
		dir, err := filepath.Abs(p)
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		query = "path:\"" + dir + "\" AND " + name
	}

	fmt.Println(query)

	//	fmt.Println("searching " + name)
	req := esapi.SearchRequest{
		Index:          []string{GOTROVI_ES_INDEX}, // Index name
		Query:          query,
		TrackTotalHits: true,
		Source:         []string{"filename", "fullname", "fullpath", "path", "size", "isfolder", "date", "extension", "hash", "mode"},
		Scroll:         59 * time.Microsecond,
		//DocvalueFields: []string{"filename", "fullname", "fullpath", "path", "size", "isfolder", "date", "extension", "hash"},
	}
	Trace.Println(req)
	res, err := req.Do(context.Background(), gotrovi.es)
	if err != nil {
		Error.Println("Error getting response:", err)
		os.Exit(1)
	}
	defer res.Body.Close()

	Trace.Println(res)

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}

	if res.IsError() {
		Error.Println("ES returned Error", res)
		Error.Println(string(body))
		os.Exit(1)
	}

	Trace.Println(string(body))

	var data SearchResult
	err = json.Unmarshal(body, &data)
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}
	total := data.Hits.Total.Value

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "Found: %d entries\n", total)

	for _, element := range data.Hits.Hits {
		PrintEntry(element.Source, element.Score, &buf)
		total = total - 1
	}
	for ok := total > 0; ok; ok = total > 0 {
		scroll := esapi.ScrollRequest{
			Scroll:   59 * time.Microsecond,
			ScrollID: data.ScrollId,
		}
		Trace.Println(scroll)
		res, err := scroll.Do(context.Background(), gotrovi.es)
		if err != nil {
			Error.Println("Error getting response:", err)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		if res.IsError() {
			Error.Println("ES returned Error", res)
			Error.Println(string(body))
			os.Exit(1)
		}

		Trace.Println(string(body))

		var data SearchResult
		err = json.Unmarshal(body, &data)
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		for _, element := range data.Hits.Hits {
			PrintEntry(element.Source, element.Score, &buf)
			total = total - 1
		}

	}

	cmd := exec.Command("less", "-X", "-N", "-r", "-S")
	cmd.Stdin = strings.NewReader(buf.String())
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}
}
