package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/jddwoody/album/internal/album"
)

func main() {
	configFilename := "config.json"
	in, err := os.Open(configFilename)
	// if we os.Open returns an error then handle it
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not open %s: %v", configFilename, err))
	}

	defer in.Close()
	bytes, _ := ioutil.ReadAll(in)
	var app album.App
	json.Unmarshal(bytes, &app)

	album := album.Album{App: app}
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", app.Port), album))
}
