package main
import (
    "encoding/json"
    "fmt"
    "io/ioutil"
	"os"
	"log"
	"io"

	"github.com/pborman/getopt"
)


const CONFIGENV = "GOTROVI_CONF"

type GotroviConf struct {
	Index []Index 	`json:"index"`
	Exclude Exclude `json:"exclude"`
}
type Index struct {
	Folder string 		`json:"folder"`
	Exclude []string 	`json:"exclude"`
}
type Exclude struct {
	Extension []string 	`json:"extension"`
	Size int 			`json:"size"`
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
    optWarning := getopt.BoolLong("warning", 'w', "Enable warnings")
    optTraces := getopt.BoolLong("traces", 't', "Enable traces")

	getopt.Parse()

    if *optHelp {
        getopt.Usage()
        os.Exit(0)
	}

	w := ioutil.Discard
	if *optWarning {
		w = os.Stdout
	}

	t := ioutil.Discard
	if *optTraces {
		t = os.Stdout
	}

	InitLogs(t, os.Stdout, w, os.Stderr)
	
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
	}

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our conf structure
	var conf GotroviConf

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'conf' which we defined above
	json.Unmarshal(byteValue, &conf)

	// we iterate through every user within our users array and
	// print out the user Type, their name, and their facebook url
	// as just an example
	for i := 0; i < len(conf.Index); i++ {
		Trace.Println("folder: " + conf.Index[i].Folder)
		for j := 0; j < len(conf.Index[i].Exclude); j++ {
			Trace.Println("exclude: " + conf.Index[i].Exclude[j])
		}
	}
	for i := 0; i < len(conf.Exclude.Extension); i++ {
		Trace.Println("exclude extensions: " + conf.Exclude.Extension[i])
	}
	Trace.Println("exclude size: ", conf.Exclude.Size)

	fmt.Println("Done")

}