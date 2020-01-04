package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const CONFIG_JSON = `{
    "index": [
      { 
        "folder": "%s",
        "exclude": [ "%s" ]
      }
    ],
    "exclude": {
        "extension": [ ".o", ".bin", ".elf", ".zip", ".jpg", ".avi", ".mkv" ],
        "folder": [ ".git", ".svn" ],
        "size": 1000000
    },
    "hash": "md5",
    "elasticsearch": {
      "host": "localhost",
      "port": 9200
    }
}`

const ES_RETRY_TIME = 1
const ES_RETRY_COUNT = 30

func (gotrovi *Gotrovi) Install() {
	// 1. Check for gotrovi config folder and create it if not present
	// 2. Check for gotrovi config file and create it if not present
	// 3. Check if Elasticsearch is running and launch it if not

	fmt.Println("Installing Gotrovi")
	Info.Println("1. Checking if config folder exists")

	path := ""

	c, exist := os.LookupEnv(CONFIGENV)

	if exist {
		Trace.Println("Environment variable " + CONFIGENV + " exists. Path: " + c)
		path = c
	} else {
		Trace.Println("Environment variable " + CONFIGENV + " does not exist. Path: " + GOTROVI_SETTINGS_FOLDER)
		path = GOTROVI_SETTINGS_FOLDER
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		Trace.Println("Folder does not exist, creating " + path)
		// create folder
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			Error.Println("Error creating folder: ")
			Error.Println(err)
			os.Exit(1)
		}
	} else {
		if !info.IsDir() {
			Error.Println("Cannot install in " + c + ". It is not a folder.")
			os.Exit(1)
		}
	}

	Info.Println("2. Checking if config file exists")

	info, err = os.Stat(path + "/" + CONFIG_FILENAME)
	if os.IsNotExist(err) {
		// create file

		usr, err := user.Current()
		if err != nil {
			Error.Println("Error getting current user info: ")
			Error.Println(err)
			os.Exit(1)
		}

		dir, err := filepath.Abs(path)
		if err != nil {
			Error.Println("Unable to get gotrovi folder absolute path: ")
			Error.Println(err)
			os.Exit(1)
		}

		Trace.Println("Creating file " + path + CONFIG_FILENAME)
		conf_file := fmt.Sprintf(CONFIG_JSON, usr.HomeDir, dir)
		err = ioutil.WriteFile(path+CONFIG_FILENAME, []byte(conf_file), os.ModePerm)
		if err != nil {
			Error.Println("Unable to create " + path + CONFIG_FILENAME)
			os.Exit(1)
		}
	}

	err = gotrovi.ParseConfig()
	if err != nil {
		Error.Println("Exiting")
		os.Exit(1)
	}

	Info.Println("3. Check if Elasticsearch is running and launch it if not")
	err = gotrovi.ConnectElasticSearch()
	if err != nil {

		info, err := os.Stat(path + "/es_data")
		if os.IsNotExist(err) {
			Trace.Println("Folder does not exist, creating " + path + "/es_data")
			// create folder
			err := os.Mkdir(path+"/es_data", os.ModePerm)
			if err != nil {
				Error.Println("Error creating folder: ")
				Error.Println(err)
				os.Exit(1)
			}
		} else {
			if !info.IsDir() {
				Error.Println("Cannot use es_data. It is not a folder.")
				os.Exit(1)
			}
		}

		// launch elasticsearch
		Info.Println("ElasticSearch is not running")

		ctx := context.Background()
		cli, err := client.NewEnvClient()
		//client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		// github packages require authenticated user to pull even public containers, so use docker hub for now
		// imageName := "docker.pkg.github.com/desordenado77/gotrovi-dockerfiles/gotrovi-es:7.4.2"
		imageName := "docker.io/gorkagarcia/gotrovi-es:latest"

		out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}
		//		io.Copy(os.Stdout, out)
		buf := new(bytes.Buffer)
		buf.ReadFrom(out)
		newStr := buf.String()

		Trace.Println(newStr)

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image: imageName,
			Env:   []string{"discovery.type=single-node"},
		},
			&container.HostConfig{
				PortBindings: nat.PortMap{
					nat.Port("9200/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9200"}},
					nat.Port("9300/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9300"}},
				},
				Binds: []string{
					path + "es_data:/usr/share/elasticsearch/data",
				},
			}, nil, "")

		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			Error.Println(err)
			os.Exit(1)
		}

		Trace.Println(resp.ID)

		fmt.Print("Waiting for ElasticSearch to be up")
		for retry := ES_RETRY_COUNT; retry > 0 && nil != gotrovi.ConnectElasticSearch(); {
			time.Sleep(ES_RETRY_TIME * time.Second)
			Trace.Println("ElasticSearch is not up yet, retrying")
			retry = retry - 1
			fmt.Print(".")
		}

		fmt.Println(" Ready!!")
		err = gotrovi.ConnectElasticSearch()
		if err != nil {
			Error.Println("Unable to get ElasticSearch running. Error: ")
			Error.Println(err)
			os.Exit(1)
		}

	}

	fmt.Println("Done. Enjoy Gotrovi now")
	Info.Println("Install Done")
}
