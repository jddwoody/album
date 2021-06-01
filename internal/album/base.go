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
	"strings"
)

type App struct {
	Port     uint32
	BodyArgs string
	Albums   map[string]Config
}

type Config struct {
	AlbumTitle          string
	AlbumDir            string
	BodyArgs            string
	ThumbNailUse        string
	ThumbNailWidth      uint32
	ThumbNailAspect     string
	ThumbDir            string
	DefaultBrowserWidth uint32
	SlideShowDelay      uint32
	NumberOfColumns     uint8
	OutsideTableBorder  uint8
	InsideTableBorder   uint8
	EditMode            bool
	AllowFinalResize    bool
	ReverseDirs         bool
	ReversePics         bool
}

type TemplateSource struct {
	App             App
	Current         Config
	Root            string
	BasePath        string
	PathInfo        string
	NumberOfColumns uint32
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
	App App
}

var (
	prefixMap = map[string]string{
		"sm":  "640x480_",
		"med": "800x600_",
		"lg":  "1024x768_",
	}
)

func (a App) String() string {
	return fmt.Sprintf(`App:{Port:%d,BodyArgs:%s,Albums:%v`, a.Port, a.BodyArgs, a.Albums)
}

func (c Config) GetThumbnailUse() string {
	if c.ThumbNailUse == "" {
		return "width"
	}
	return c.ThumbNailUse
}

func (c Config) GetThumbnailWidth() uint32 {
	if c.ThumbNailWidth == 0 {
		return 100
	}
	return c.ThumbNailWidth
}

func (c Config) GetDefaultBrowserWidth() uint32 {
	if c.DefaultBrowserWidth == 0 {
		return 640
	}
	return c.DefaultBrowserWidth
}

func (c Config) String() string {
	return fmt.Sprintf(`Config:{AlbumTitle:"%s",AlbumDir:"%s",BodyArgs:%s,ThumbNailUse:"%s",ThumbNailWidth:%d,ThumbNailAspect:"%s",ThumbDir:"%s",SlideShowDelay:%d,NumberOfColumns:%d,EditMode:%v,AllowFinalResize:%v,ReverseDirs:%v,ReversePics:%v`,
		c.AlbumTitle, c.AlbumDir, c.BodyArgs, c.ThumbNailUse, c.ThumbNailWidth, c.ThumbNailAspect, c.ThumbDir,
		c.SlideShowDelay, c.NumberOfColumns, c.EditMode, c.AllowFinalResize, c.ReverseDirs, c.ReversePics)
}

func (t TemplateSource) String() string {
	return fmt.Sprintf(`TemplateSource:{App:%v,Current:%v,Root:%s,BasePath:%s,PathInfo:%s,Files:%v,Dirs:%v,PageTitle:%s,ActualPath:%s,BaseFilename:%s,FileIndex:%d,PrevSeven:%s,NextSeven:%s,CaptionHtml:%s,CaptionMap:%v`,
		t.App, t.Current, t.Root, t.BasePath, t.PathInfo, t.Files, t.Dirs, t.PageTitle, t.ActualPath, t.BaseFilename, t.FileIndex, t.PrevSeven, t.NextSeven, t.CaptionHtml, t.CaptionMap)
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
