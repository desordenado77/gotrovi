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

type Highlight struct {
	Field []string `json:"attachment.content"`
}

type SearchHit struct {
	Score     float64   `json:"_score"`
	Source    Source    `json:"_source"`
	Highlight Highlight `json:"highlight"`
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

func PrintEntry(e SearchHit, bScore bool, sHighligh string, buf *bytes.Buffer) {
	s := e.Source
	score := e.Score

	highlightColorfn := color.FgRed.Render
	colorfn := color.FgMagenta.Render

	if s.IsFolder {
		colorfn = color.FgBlue.Render
	} else {
		if strings.Contains(s.Mode, "x") {
			colorfn = color.FgGreen.Render
		}
	}
	if len(e.Highlight.Field) == 0 {
		fmt.Fprintf(buf, "%s\n", colorfn(s.FullName))
	} else {
		for _, element := range e.Highlight.Field {
			fmt.Fprintf(buf, "%s:%s\n", colorfn(s.FullName), strings.Replace(element, sHighligh, highlightColorfn(sHighligh), -1))
		}
	}

	if bScore {
		fmt.Fprintf(buf, "Score: %g\n", score)
	}
}

func (gotrovi *Gotrovi) Find(name string, p []string, s bool, h string, hb bool) {
	query := name

	if len(p) != 0 {
		/*		var err error
				dir, err = os.Getwd()
				if err != nil {
					Error.Println(err)
					os.Exit(1)
				}
		*/
		dir_query := "("
		for i, element := range p {
			dir, err := filepath.Abs(element)
			if err != nil {
				Error.Println(err)
				os.Exit(1)
			}

			dir_query = dir_query + "path:\"" + dir + "\""

			if i != (len(p) - 1) {
				dir_query = dir_query + " OR "
			}
		}
		dir_query = dir_query + ")"
		query = dir_query + " AND " + query
	}

	Trace.Println(query)

	highlighter := ""
	fmt.Println(h)
	if h != "" || hb {
		highlighter = "{ \"highlight\" : { \"fields\" : { \"attachment.content\" : {} } } }"
	}

	//	fmt.Println("searching " + name)
	req := esapi.SearchRequest{
		Index:          []string{GOTROVI_ES_INDEX}, // Index name
		Query:          query,
		TrackTotalHits: true,
		Source:         []string{"filename", "fullname", "fullpath", "path", "size", "isfolder", "date", "extension", "hash", "mode"},
		Scroll:         59 * time.Microsecond,
		Body:           strings.NewReader(highlighter),
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
		PrintEntry(element, s, h, &buf)
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
			PrintEntry(element, s, h, &buf)
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
