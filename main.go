package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch"
	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/pborman/getopt"
)

const CONFIGENV = "GOTROVI_CONF"
const GOTROVI_ES_INDEX = "gotrovi"

type GotroviConf struct {
	Index         []Index  `json:"index"`
	Exclude       Exclude  `json:"exclude"`
	Hash          string   `json:"hash"`
	ElasticSearch ESConfig `json:"elasticsearch"`
}
type Index struct {
	Folder  string   `json:"folder"`
	Exclude []string `json:"exclude"`
}
type Exclude struct {
	Extension []string `json:"extension"`
	Folder    []string `json:"folder"`
	Size      int64    `json:"size"`
}
type ESConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type FileDescriptionDoc struct {
	FileName  string `json:"filename"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	Hash      string `json:"hash"`
	Data      string `json:"data"`
	IsFolder  bool   `json:"isfolder"`
	Date      string `json:"date"`
}

type Gotrovi struct {
	conf  GotroviConf
	count int
	hash  hash.Hash
	es    *elasticsearch.Client
}

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func InitLogs(
	traceHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	Trace = log.New(traceHandle,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
	//    optName := getopt.StringLong("name", 'n', "Torpedo", "Your name")
	optHelp := getopt.BoolLong("help", 'h', "Show this message")
	optVerbose := getopt.IntLong("verbose", 'v', 0, "Set verbosity: 0 to 3")
	optSync := getopt.BoolLong("Sync", 's', "Perform Sync")

	getopt.Parse()

	if *optHelp {
		getopt.Usage()
		os.Exit(0)
	}

	vw := ioutil.Discard
	if *optVerbose > 0 {
		vw = os.Stdout
	}

	vi := ioutil.Discard
	if *optVerbose > 1 {
		vi = os.Stdout
	}

	vt := ioutil.Discard
	if *optVerbose > 2 {
		vt = os.Stdout
	}

	InitLogs(vt, vi, vw, os.Stderr)

	// read config file from:
	// 1. .gotrovi/config.json in ~/
	// 2. GOTROVI_CONF env variable
	// 3. in ./config.json

	// Open our jsonFile
	jsonFile, err := os.Open("~/.gotrovi/config.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		Warning.Println(err)

		c, exist := os.LookupEnv(CONFIGENV)

		if exist {
			jsonFile, err = os.Open(c)
			if err != nil {
				Warning.Println(err)
			}
		} else {
			Warning.Println(CONFIGENV + " environment variable not found")
			err = os.ErrNotExist
		}

		if err != nil {
			jsonFile, err = os.Open("./config.json")
			if err != nil {
				Warning.Println(err)
			}
		}
	}

	if err != nil {
		Error.Println("Unable to open config file")
		os.Exit(1)
	}

	// read our opened xmlFile as a byte array.
	byteValue, err := ioutil.ReadAll(jsonFile)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		os.Exit(1)
	}

	// we initialize our conf structure
	var gotrovi Gotrovi

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'conf' which we defined above
	err = json.Unmarshal(byteValue, &gotrovi.conf)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		os.Exit(1)
	}

	// we iterate through every user within our users array and
	// print out the user Type, their name, and their facebook url
	// as just an example
	for i := 0; i < len(gotrovi.conf.Index); i++ {
		Trace.Println("folder: " + gotrovi.conf.Index[i].Folder)
		for j := 0; j < len(gotrovi.conf.Index[i].Exclude); j++ {
			Trace.Println("exclude: " + gotrovi.conf.Index[i].Exclude[j])
		}
	}
	for i := 0; i < len(gotrovi.conf.Exclude.Extension); i++ {
		Trace.Println("exclude extensions: " + gotrovi.conf.Exclude.Extension[i])
	}
	Trace.Println("exclude size: ", gotrovi.conf.Exclude.Size)

	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://" + gotrovi.conf.ElasticSearch.Host + ":" + strconv.Itoa(gotrovi.conf.ElasticSearch.Port),
		},
	}

	gotrovi.es, err = elasticsearch.NewClient(cfg)

	if err != nil {
		Error.Println("Error connecting to ElasticSearch " + cfg.Addresses[0])
		Error.Println(err)
		os.Exit(1)
	}
	res, err := gotrovi.es.Info()

	Trace.Println(res)

	if err != nil {
		Error.Println("Error connecting to ElasticSearch " + cfg.Addresses[0])
		Error.Println(err)
		os.Exit(1)
	}

	// configure Elastic
	body := "{ \"processors\" : [ { \"attachment\" : { \"field\" : \"data\" }, \"remove\": { \"field\": \"data\" } } ] }"

	req := esapi.IngestPutPipelineRequest{DocumentID: "attachement", Body: strings.NewReader(body)}
	res, err = req.Do(context.Background(), gotrovi.es)
	if err != nil {
		Error.Println(err)
	}
	defer res.Body.Close()

	if res.IsError() {
		Error.Println("Unable to set pipeline attachement")
		Error.Println(err)
		os.Exit(1)
	}

	if *optSync {
		gotrovi.Sync()
	}

	fmt.Println("Done")
}
