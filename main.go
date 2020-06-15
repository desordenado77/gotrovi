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
	"os/user"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/apoorvam/goterminal"
	"github.com/elastic/go-elasticsearch"
	"github.com/pborman/getopt"
)

const CONFIGENV = "GOTROVI_CONF"
const GOTROVI_ES_INDEX = "gotrovi"
const CONFIG_FILENAME = "config.json"
const GOTROVI_SETTINGS_FOLDER_PATTERN = "%s/.gotrovi/"

var GOTROVI_SETTINGS_FOLDER string

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

	wg   sync.WaitGroup
	wait int
	jobs int
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

func (gotrovi *Gotrovi) ConnectElasticSearch() (err error) {

	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://" + gotrovi.conf.ElasticSearch.Host + ":" + strconv.Itoa(gotrovi.conf.ElasticSearch.Port),
		},
	}

	gotrovi.es, err = elasticsearch.NewClient(cfg)

	if err != nil {
		Error.Println("Error connecting to ElasticSearch " + cfg.Addresses[0])
		Error.Println(err)
		return err
	}
	res, err := gotrovi.es.Info()

	Trace.Println(res)

	if err != nil {
		Trace.Println("Error connecting to ElasticSearch " + cfg.Addresses[0])
		Trace.Println(err)
		return err
	}

	return nil
}

func (gotrovi *Gotrovi) ParseConfig() (err error) {

	// read config file from:
	// 1. GOTROVI_CONF env variable
	// 2. .gotrovi/config.json in ~/
	// 3. in ./config.json

	var jsonFile *os.File

	c, exist := os.LookupEnv(CONFIGENV)

	if exist {
		jsonFile, err = os.Open(c + "/" + CONFIG_FILENAME)
		if err != nil {
			Warning.Println(err)
		}
	} else {
		Warning.Println(CONFIGENV + " environment variable not found")
		err = os.ErrNotExist
	}

	if err != nil {
		jsonFile, err = os.Open(GOTROVI_SETTINGS_FOLDER + CONFIG_FILENAME)
		if err != nil {
			Warning.Println(err)
			jsonFile, err = os.Open("./" + CONFIG_FILENAME)
			if err != nil {
				Warning.Println(err)
			}
		}
	}

	if err != nil {
		Error.Println("Unable to open config file")
		return err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		return err
	}

	err = json.Unmarshal(byteValue, &gotrovi.conf)

	if err != nil {
		Error.Println("Unable to read config file: " + jsonFile.Name())
		return err
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

	return nil
}

func main() {

	InitLogs(os.Stdout, os.Stdout, os.Stdout, os.Stderr)

	usr, err := user.Current()
	if err != nil {
		Error.Println("Error getting current user info: ")
		Error.Println(err)
		os.Exit(1)
	}

	GOTROVI_SETTINGS_FOLDER = fmt.Sprintf(GOTROVI_SETTINGS_FOLDER_PATTERN, usr.HomeDir)

	getopt.SetUsage(usage)

	//    optName := getopt.StringLong("name", 'n', "Torpedo", "Your name")
	optHelp := getopt.BoolLong("help", 'h', "Show this message")
	optVerbose := getopt.IntLong("verbose", 'v', 0, "Set verbosity: 0 to 3")
	optSync := getopt.StringLong("sync", 's', "", "Perform Sync. Options:\n\"forced\" this is the brute force sync type in which the ES index is deleted and the whole FS is processed\n\"update\" update existing documents in Elasticsearch\n\"updateFast\" same as update, only slightly faster")
	optFind := getopt.StringLong("find", 'f', "", "Find file by name")
	optScore := getopt.BoolLong("score", 'c', "Display elasticsearch score in searches")
	optDelete := getopt.BoolLong("delete", 'd', "Delete elasticsearch index")
	optHighlightString := getopt.StringLong("grep", 'g', "", "Grep style output showing the match in the content. Give the text to grep for in the highlights as parameter")
	optHighlightBool := getopt.BoolLong("Grep", 'G', "Grep style output showing the match in the content")
	optInstall := getopt.BoolLong("install", 'i', "Install the necessary config files in "+GOTROVI_SETTINGS_FOLDER+" and run the Elasticsearch container")
	optJobs := getopt.IntLong("jobs", 'j', 32, "Set amount of sync jobs. Default is 32")
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

	var gotrovi Gotrovi

	InitLogs(vt, vi, vw, os.Stderr)

	gotrovi.jobs = *optJobs

	if *optInstall {
		Info.Println("Insalling gotrovi")
		gotrovi.Install()
	}

	err = gotrovi.ParseConfig()
	if err != nil {
		Error.Println("Exiting")
		os.Exit(1)
	}

	err = gotrovi.ConnectElasticSearch()
	if err != nil {
		Error.Println("Unable to connect with Elasticsearch. Error: ")
		Error.Println(err)
		os.Exit(1)
	}

	if *optDelete {
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("Are you shure you want to delete the \"" + GOTROVI_ES_INDEX + "\" index? (y/n)")
			text, _ := reader.ReadString('\n')
			text = strings.Replace(strings.ToLower(text), "\n", "", -1)
			if text == "yes" || text == "y" {
				gotrovi.DeleteIndex()
				fmt.Println("Index \"" + GOTROVI_ES_INDEX + "\" deleted")
				break
			} else if text == "no" || text == "n" {
				break
			}
		}
	}

	gotrovi.writer = goterminal.New(os.Stdout)

	if *optSync != "" {
		Info.Println("Using", gotrovi.jobs, "jobs")
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
