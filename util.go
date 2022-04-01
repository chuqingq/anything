package main

import (
	"encoding/json"
	"os"
)

// database read and write
const dbFile = "./anything.db"

func readDB(files *[]*File) error {
	f, err := os.OpenFile(dbFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	return decoder.Decode(files)
}

func writeDB(files []*File) error {
	f, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	return encoder.Encode(files)
}
