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
	"time"

	"github.com/jddwoody/album/internal/album"
)

func main() {
	if !album.IsFfmpegAvailable() {
		fmt.Println("ffmpeg is not available, no video support")
	}

	app, err := album.LoadAppConfigFile()
	if err != nil {
		log.Fatalf("Error loading config file %s, err:%v", album.APP_CONFIG_FILENAME, err)
	}
	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", app.Port),
		Handler:           album.Album{},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
