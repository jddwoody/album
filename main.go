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

	"github.com/jddwoody/album/internal/album"
)

func main() {
	app, err := album.LoadConfigFile()
	if err != nil {
		log.Fatal(fmt.Sprintf("Error loading config file %s, err:%v", album.CONFIG_FILENAME, err))
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", app.Port), album.Album{}))
}
