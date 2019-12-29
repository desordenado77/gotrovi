package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/apoorvam/goterminal"
	"github.com/elastic/go-elasticsearch"
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
	FullName  string `json:"fullpath"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	Hash      string `json:"hash"`
	Data      string `json:"data"`
	IsFolder  bool   `json:"isfolder"`
	Date      string `json:"date"`
	Mode      string `json:"mode"`
}

type Gotrovi struct {
	conf   GotroviConf
	count  int
	total  int
	added  int
	hash   hash.Hash
	es     *elasticsearch.Client
	writer *goterminal.Writer
	//	stdscr *gc.Window
}

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func usage() {
	w := os.Stdout

	getopt.PrintUsage(w)
	fmt.Printf("\n[parameters ...] may contain paths to restrict the search to. You may also use lucene queries to do the same, but this is more convenient.\n")
	fmt.Printf("You may search for the following fields: \n\t")

	var b FileDescriptionDoc
	val := reflect.ValueOf(b)
	for i := 0; i < val.Type().NumField(); i++ {
		fmt.Printf("%s, ", val.Type().Field(i).Tag.Get("json"))
	}
	fmt.Println("attachment.content, attachment.content_type, attachment.language")

	fmt.Printf("\nExamples:\n")

	fmt.Printf("\tFind files bigger than 10 bytes named test\n")
	fmt.Printf("\t\tgotrovi -f \"size:>=10 AND filename:test\"\n\n")
	fmt.Printf("\tFind folders named test\n")
	fmt.Printf("\t\tgotrovi -f \"filename:test AND isfolder:true\"\n\n")
	fmt.Printf("\tFind files containing test\n")
	fmt.Printf("\t\tgotrovi -f \"attachment.content:test\"\n\n")
	fmt.Println("More info on the syntax used to find files in the Lucene query documentation: https://lucene.apache.org/core/2_9_4/queryparsersyntax.html")
}

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
	getopt.SetUsage(usage)

	//    optName := getopt.StringLong("name", 'n', "Torpedo", "Your name")
	optHelp := getopt.BoolLong("Help", 'h', "Show this message")
	optVerbose := getopt.IntLong("Verbose", 'v', 0, "Set verbosity: 0 to 3")
	optSync := getopt.StringLong("Sync", 's', "", "Perform Sync. force is the brute force sync type in which the ES index is deleted and the whole FS is processed")
	optFind := getopt.StringLong("Find", 'f', "", "Find file by name")
	optScore := getopt.BoolLong("sCore", 'c', "Display elasticseach score")
	optHighlightString := getopt.StringLong("grep", 'g', "", "Grep style output showing the match in the content. Give the text to grep for in the highlights as parameter")
	optHighlightBool := getopt.BoolLong("Grep", 'G', "Grep style output showing the match in the content")
	var searchPath []string

	getopt.Parse()

	if len(getopt.Args()) != 0 {
		searchPath = getopt.Args()
	}

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

	byteValue, err := ioutil.ReadAll(jsonFile)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		os.Exit(1)
	}

	var gotrovi Gotrovi

	err = json.Unmarshal(byteValue, &gotrovi.conf)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		os.Exit(1)
	}

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

	gotrovi.writer = goterminal.New(os.Stdout)

	if *optSync != "" {
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("Are you shure you want to resynch? (y/n)")
			text, _ := reader.ReadString('\n')
			text = strings.Replace(strings.ToLower(text), "\n", "", -1)
			if text == "yes" || text == "y" {

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

				if *optSync == "forced" {
					gotrovi.SyncForced()
				}
				if *optSync == "update" {
					gotrovi.SyncUpdate(true)
					gotrovi.SyncAddMissing()
				}
				if *optSync == "updateFast" {
					gotrovi.SyncUpdate(false)
					gotrovi.SyncAddMissing()
				}
				break
			} else if text == "no" || text == "n" {
				break
			}
		}
	}

	if *optFind != "" {
		gotrovi.Find(*optFind, searchPath, *optScore, *optHighlightString, *optHighlightBool)
	}
}
