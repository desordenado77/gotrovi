package main

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/esapi"
)

func (gotrovi *Gotrovi) Find(name string) {
	fmt.Println("searching " + name)
	req := esapi.SearchRequest{
		Index: []string{GOTROVI_ES_INDEX}, // Index name
		Query: name,
	}
	Trace.Println(req)
	res, err := req.Do(context.Background(), gotrovi.es)
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
