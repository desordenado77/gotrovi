package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/elastic/go-elasticsearch/esapi"
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
}

type SearchHit struct {
	Source Source `json:"_source"`
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

/*
{
    "took": 1,
    "timed_out": false,
    "_shards":{
        "total" : 1,
        "successful" : 1,
        "skipped" : 0,
        "failed" : 0
    },
    "hits":{
        "total" : {
            "value": 1,
            "relation": "eq"
        },
        "max_score": 1.3862944,
        "hits" : [
            {
                "_index" : "twitter",
                "_type" : "_doc",
                "_id" : "0",
                "_score": 1.3862944,
                "_source" : {
                    "user" : "kimchy",
                    "message": "trying out Elasticsearch",
                    "date" : "2009-11-15T14:12:12",
                    "likes" : 0
                }
            }
        ]
    }
}
*/
func (gotrovi *Gotrovi) Find(name string) {
	fmt.Println("searching " + name)
	req := esapi.SearchRequest{
		Index:          []string{GOTROVI_ES_INDEX}, // Index name
		Query:          name,
		TrackTotalHits: true,
		Source:         []string{"filename", "fullname", "fullpath", "path", "size", "isfolder", "date", "extension", "hash"},
		Scroll:         59 * time.Microsecond,
		//DocvalueFields: []string{"filename", "fullname", "fullpath", "path", "size", "isfolder", "date", "extension", "hash"},
	}
	Trace.Println(req)
	res, err := req.Do(context.Background(), gotrovi.es)
	if err != nil {
		Error.Println("Error getting response:", err)
		return
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
	for _, element := range data.Hits.Hits {
		// element is the element from someSlice for where we are
		fmt.Println(strconv.Itoa(data.Hits.Total.Value-total) + " " + element.Source.FullName)
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
			// element is the element from someSlice for where we are
			fmt.Println(strconv.Itoa(data.Hits.Total.Value-total) + " " + element.Source.FullName)
			total = total - 1
		}
	}

	//	fmt.Println(data)

	// this is where the magic happens, I pass a pointer of type Person and Go'll do the rest
	/*	err = json.Unmarshal(body, &p)

		if err != nil {
			panic(err)
		}

		fmt.Println(p.Name) // Jhon
		fmt.Println(p.Age)  // 87
	*/
}
