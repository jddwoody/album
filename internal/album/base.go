package album

/*
   Copyright 1998-2021 James D Woodgate.  All rights reserved.
   It may be used and modified freely, but I do request that this copyright
   notice remain attached to the file.  You may modify this module as you
   wish, but if you redistribute a modified version, please attach a note
   listing the modifications you have made.
*/

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DEFAULT_ASPECT  = 0.2
	CONFIG_FILENAME = "config.yaml"
)

type App struct {
	Port      int               `yaml:"port"`
	BodyArgs  string            `yaml:"bodyArgs"`
	Default   Config            `yaml:"default"`
	Albums    map[string]Config `yaml:"albums"`
	Timestamp time.Time         `yaml:"timestamp"`
}

// Implement aspect ratio

type Config struct {
	AlbumTitle          string `yaml:"albumTitle"`
	AlbumDir            string `yaml:"albumDir"`
	BodyArgs            string `yaml:"bodyArgs"`
	ThumbnailUse        string `yaml:"thumbnailUse"`
	ThumbnailWidth      int    `yaml:"thumbnailWidth"`
	ThumbnailAspect     string `yaml:"thumbnailAspect"`
	ThumbDir            string `yaml:"thumbDir"`
	DefaultBrowserWidth int    `yaml:"defaultBrowserWidth"`
	SlideShowDelay      int    `yaml:"slideShowDelay"`
	NumberOfColumns     int    `yaml:"numberOfColumns"`
	OutsideTableBorder  int    `yaml:"outsideTableBorder"`
	InsideTableBorder   int    `yaml:"insideTableBorder"`
	EditMode            bool   `yaml:"editMode"`
	AllowFinalResize    bool   `yaml:"allowFinalResize"`
	ReverseDirs         bool   `yaml:"reverseDirs"`
	ReversePics         bool   `yaml:"reversePics"`
}

type TemplateSource struct {
	App             *App
	Current         Config
	Root            string
	BasePath        string
	PathInfo        string
	NumberOfColumns int
	Files           []os.FileInfo
	Dirs            []os.FileInfo
	PageTitle       string
	ActualPath      string
	BaseFilename    string
	FileIndex       int
	PrevSeven       string
	NextSeven       string
	CaptionHtml     string
	CaptionMap      map[string]string
}

type CaptionFile struct {
	Html       string
	CaptionMap map[string]string
}

type Album struct {
}

type AlbumTitle struct {
	Key   string
	Title string
}

var (
	prefixMap = map[string]string{
		"sm":  "640x480_",
		"med": "800x600_",
		"lg":  "1024x768_",
	}
)

func (a App) String() string {
	return fmt.Sprintf(`App:{Port:%d,BodyArgs:%s,Default:%s,Albums:%v,Timestamp:%v`, a.Port, a.BodyArgs, a.Default, a.Albums, a.Timestamp)
}

func (c Config) GetThumbnailUse() string {
	if c.ThumbnailUse == "" {
		return "width"
	}
	return c.ThumbnailUse
}

func (c Config) GetThumbnailWidth() int {
	if c.ThumbnailWidth == 0 {
		return 100
	}
	return c.ThumbnailWidth
}

func (c Config) GetDefaultBrowserWidth() int {
	if c.DefaultBrowserWidth == 0 {
		return 640
	}
	return c.DefaultBrowserWidth
}

func (c Config) GetThumbnailAspect() float64 {
	if c.ThumbnailAspect == "" {
		return DEFAULT_ASPECT
	}

	aspect, err := strconv.ParseFloat(strings.TrimSpace(c.ThumbnailAspect), 64)
	if err == nil {
		return aspect
	}

	// If it's not already a float see if it's a simple division
	split := strings.Split(c.ThumbnailAspect, "/")
	if len(split) != 2 {
		return DEFAULT_ASPECT
	}

	num, err := strconv.ParseFloat(strings.TrimSpace(split[0]), 64)
	if err != nil {
		fmt.Printf("Could not parse:'%s', err:%v\n", split[0], err)
		return DEFAULT_ASPECT
	}
	den, err := strconv.ParseFloat(strings.TrimSpace(split[1]), 64)
	if err != nil {
		fmt.Printf("Could not parse:'%s', err:%v\n", split[1], err)
		return DEFAULT_ASPECT
	}
	return num / den
}

func (c Config) String() string {
	return fmt.Sprintf(`Config:{AlbumTitle:"%s",AlbumDir:"%s",BodyArgs:%s,ThumbnailUse:"%s",ThumbnailWidth:%d,ThumbnailAspect:"%s",ThumbDir:"%s",SlideShowDelay:%d,NumberOfColumns:%d,EditMode:%v,AllowFinalResize:%v,ReverseDirs:%v,ReversePics:%v`,
		c.AlbumTitle, c.AlbumDir, c.BodyArgs, c.ThumbnailUse, c.ThumbnailWidth, c.ThumbnailAspect, c.ThumbDir,
		c.SlideShowDelay, c.NumberOfColumns, c.EditMode, c.AllowFinalResize, c.ReverseDirs, c.ReversePics)
}

func (t TemplateSource) String() string {
	return fmt.Sprintf(`TemplateSource:{App:%v,Current:%v,Root:%s,BasePath:%s,PathInfo:%s,Files:%v,Dirs:%v,PageTitle:%s,ActualPath:%s,BaseFilename:%s,FileIndex:%d,PrevSeven:%s,NextSeven:%s,CaptionHtml:%s,CaptionMap:%v`,
		t.App, t.Current, t.Root, t.BasePath, t.PathInfo, t.Files, t.Dirs, t.PageTitle, t.ActualPath, t.BaseFilename, t.FileIndex, t.PrevSeven, t.NextSeven, t.CaptionHtml, t.CaptionMap)
}

func (a AlbumTitle) String() string {
	return fmt.Sprintf(`AlbumTitle:{Key%s, Title:%s`, a.Key, a.Title)
}

func NewCaptionFile(f io.Reader) *CaptionFile {
	html := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() && scanner.Text() != "__END__" {
		html += scanner.Text() + "\n"
	}

	captionMap := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, ":")
		if i > -1 {
			captionMap[line[0:i]] = line[i+1:]
		}
	}

	return &CaptionFile{
		Html:       html,
		CaptionMap: captionMap,
	}
}

// merges any non-default values in b into a
func Merge(a, b *Config) {
	if b.AlbumTitle != "" {
		a.AlbumTitle = b.AlbumTitle
	}
	if b.AlbumDir != "" {
		a.AlbumDir = b.AlbumDir
	}

	if b.BodyArgs != "" {
		a.BodyArgs = b.BodyArgs
	}

	if b.ThumbnailUse != "" {
		a.ThumbnailUse = b.ThumbnailUse
	}

	if b.ThumbnailWidth > 0 {
		a.ThumbnailWidth = b.ThumbnailWidth
	}

	if b.ThumbnailAspect != "" {
		a.ThumbnailAspect = b.ThumbnailAspect
	}

	if b.ThumbDir != "" {
		a.ThumbDir = b.ThumbDir
	}

	if b.SlideShowDelay > 0 {
		a.SlideShowDelay = b.SlideShowDelay
	}

	if b.NumberOfColumns > 0 {
		a.NumberOfColumns = b.NumberOfColumns
	}

	if b.EditMode {
		a.EditMode = true
	}

	if b.AllowFinalResize {
		a.AllowFinalResize = true
	}
	if b.ReverseDirs {
		a.ReverseDirs = true
	}

	if b.ReversePics {
		a.ReversePics = true
	}
}
