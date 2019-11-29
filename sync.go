package main

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"os"

	"path/filepath"
)

type folderOperation func(*Gotrovi, os.FileInfo, string)

func (gotrovi *Gotrovi) Sync() {
	Info.Println("Performing Sync")

	for i := 0; i < len(gotrovi.conf.Index); i++ {
		gotrovi.SyncFolder(i)
	}
}

func (gotrovi *Gotrovi) SyncFolder(i int) {
	f := gotrovi.conf.Index[i].Folder
	Info.Println("- " + f)
	gotrovi.count = 0
	gotrovi.PerformFolderOperation(i, count)
	Info.Println("Found files: ", gotrovi.count)

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

	gotrovi.PerformFolderOperation(i, sync)

}

func count(g *Gotrovi, info os.FileInfo, p string) {
	g.count = g.count + 1
}

func sync(g *Gotrovi, info os.FileInfo, p string) {
	if !info.IsDir() {
		f, err := os.Open(p)
		if err != nil {
			Error.Println(err)
		}
		defer f.Close()

		if _, err := io.Copy(g.hash, f); err != nil {
			Error.Println(err)
		}
		sum := g.hash.Sum(nil)

		fmt.Printf("%x\n", sum)
	}
}

func (gotrovi *Gotrovi) PerformFolderOperation(id int, fo folderOperation) {
	f := gotrovi.conf.Index[id].Folder

	err := filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			Error.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() {
			for i := 0; i < len(gotrovi.conf.Index[id].Exclude); i++ {
				if path == gotrovi.conf.Index[id].Exclude[i] {
					Trace.Println("Skipping Folder (fullpath) " + path)
					return filepath.SkipDir
				}
			}
			for i := 0; i < len(gotrovi.conf.Exclude.Folder); i++ {
				if info.Name() == gotrovi.conf.Exclude.Folder[i] {
					Trace.Println("Skipping Folder (name) " + path)
					return filepath.SkipDir
				}
			}
		}
		// exclude extensions
		for i := 0; i < len(gotrovi.conf.Exclude.Extension); i++ {
			if filepath.Ext(path) == gotrovi.conf.Exclude.Extension[i] {
				Trace.Println("Skipping (ext) " + path)
				return nil
			}
		}

		if info.Size() > gotrovi.conf.Exclude.Size {
			Trace.Println("Skipping (size) " + path)
			return nil
		}

		fmt.Println(path)

		fo(gotrovi, info, path)

		return nil
	})
	if err != nil {
		Error.Println(err)
	}
}
