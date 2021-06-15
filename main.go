package main

/*
   Copyright 1998-2021 James D Woodgate.  All rights reserved.
   It may be used and modified freely, but I do request that this copyright
   notice remain attached to the file.  You may modify this module as you
   wish, but if you redistribute a modified version, please attach a note
   listing the modifications you have made.
*/

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jddwoody/album/internal/album"
	"gopkg.in/yaml.v2"
)

func main() {
	configFilename := "config.yaml"
	in, err := os.Open(configFilename)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not open %s: %v", configFilename, err))
	}

	defer in.Close()
	decoder := yaml.NewDecoder(in)
	var app album.App
	err = decoder.Decode(&app)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error parsing %s: %v", configFilename, err))
	}

	album := album.Album{App: app}
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", app.Port), album))
}
