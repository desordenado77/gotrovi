package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
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

func (gotrovi *Gotrovi) Install() {
	// 1. Check for gotrovi config folder and create it if not present
	// 2. Check for gotrovi config file and create it if not present
	// 3. Check if Elasticsearch is running and launch it if not

	Info.Println("Performing install")
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
	Info.Println("Install Done")
}
