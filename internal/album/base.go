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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alitto/pond"
)

const (
	DEFAULT_ASPECT         = 0.2
	APP_CONFIG_FILENAME    = "appconfig.yaml"
	ALBUMS_CONFIG_FILENAME = "albumsconfig.yaml"
	CONFIG_FILENAME        = "config.yaml"
)

type AppConfig struct {
	Port      int    `yaml:"port"`
	AlbumsDir string `yaml:"albumsDir"`
}

type AlbumsConfig struct {
	BodyArgs string                 `yaml:"bodyArgs"`
	Default  Config                 `yaml:"default"`
	Albums   map[string]AlbumConfig `yaml:"albums"`
}

type AlbumConfig struct {
	AlbumTitle string `yaml:"albumTitle"`
	AlbumDir   string `yaml:"albumDir"`
	ThumbDir   string `yaml:"thumbDir"`
	Config     Config `yaml:"config"`
}

type Config struct {
	BodyArgs            string `yaml:"bodyArgs"`
	VideoThumbnailSize  string `yaml:"videoThumbnailSize"`
	ThumbnailUse        string `yaml:"thumbnailUse"`
	ThumbnailWidth      int    `yaml:"thumbnailWidth"`
	ThumbnailAspect     string `yaml:"thumbnailAspect"`
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
	AppConfig       *AppConfig
	AlbumsConfig    *AlbumsConfig
	AlbumConfig     AlbumConfig
	Current         Config
	Root            string
	BasePath        string
	PathInfo        string
	DirInfo         string
	NumberOfColumns int
	Files           []os.DirEntry
	Dirs            []os.DirEntry
	ImageCount      int
	FullTitle       string
	PageTitle       string
	ActualPath      string
	Mp4Path         string
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

	filetypeMap = map[string][]string{
		"images":   {"jpg", "jpeg", "gif"},
		"videos":   {"avi", "mpeg"},
		"htmlview": {"ogg", "webm", "mp4", "mov"},
	}

	pool = pond.New(3, 750)

	workingMap = make(map[string]bool)
)

func GetImageFiles(files []os.DirEntry) []os.DirEntry {
	var imageFiles []os.DirEntry
	for _, file := range files {
		if IsImageFile(file.Name()) {
			imageFiles = append(imageFiles, file)
		}
	}
	return imageFiles
}

func IsViewableFile(filename string) bool {
	return IsImageFile(filename) || IsVideoFile(filename)
}

func IsImageFile(filename string) bool {
	asLower := strings.ToLower(filename)
	for _, filetype := range filetypeMap["images"] {
		if strings.HasSuffix(asLower, filetype) {
			return true
		}
	}
	return false
}

func IsVideoFile(filename string) bool {
	return CanHtmlPlay(filename) || VideoNeedsConversion(filename)
}

func VideoNeedsConversion(filename string) bool {
	asLower := strings.ToLower(filename)
	for _, filetype := range filetypeMap["videos"] {
		if strings.HasSuffix(asLower, filetype) {
			return true
		}
	}
	return false
}

func CanHtmlPlay(filename string) bool {
	asLower := strings.ToLower(filename)
	for _, filetype := range filetypeMap["htmlview"] {
		if strings.HasSuffix(asLower, filetype) {
			return true
		}
	}
	return false
}

func IsFfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func GenerateVideoThumbnail(inFilename, size, outFilename string) error {
	cmd := exec.Command("ffmpeg", "-i", inFilename, "-frames:v", "1", "-s", size, outFilename)
	return cmd.Run()
}

func ConvertVideoFile(inFilename, outFilename string) error {
	cmd := exec.Command("ffmpeg", "-i", inFilename, outFilename)
	return cmd.Run()
}

func ChangeExtension(filename, ext string) string {
	return fmt.Sprintf("%s.%s", strings.TrimSuffix(filename, filepath.Ext(filename)), ext)
}

func (a AppConfig) String() string {
	return fmt.Sprintf("AppConfig:{Port:%d,AlbumsDir:%s}", a.Port, a.AlbumsDir)
}

func (a AlbumsConfig) String() string {
	return fmt.Sprintf("AlbumsConfig:{Default:%v,Albums:%v}", a.Default, a.Albums)
}

func (a AlbumConfig) String() string {
	return fmt.Sprintf("AlbumConfig:{AlbumTitle:%s,AlbumDir:%s,ThumbDir:%s,Config:%v},", a.AlbumTitle, a.AlbumDir, a.ThumbDir, a.Config)
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

func (c Config) GetVideoThumbnailSize() string {
	if c.VideoThumbnailSize == "" {
		return "200x150"
	}
	return c.VideoThumbnailSize
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
	return fmt.Sprintf("Config:{BodyArgs:%s,VideoThumbnailSize:%s,ThumbnailUse:%s,ThumbnailWidth:%d,ThumbnailAspect:%s,SlideShowDelay:%d,NumberOfColumns:%d,EditMode:%v,AllowFinalResize:%v,ReverseDirs:%v,ReversePics:%v}",
		c.BodyArgs, c.ThumbnailUse, c.VideoThumbnailSize, c.ThumbnailWidth, c.ThumbnailAspect, c.SlideShowDelay, c.NumberOfColumns, c.EditMode, c.AllowFinalResize, c.ReverseDirs, c.ReversePics)
}

func (t TemplateSource) String() string {
	return fmt.Sprintf(`TemplateSource:{AppConfig:%v,AlbumConfig:%v,AlbumConfig:%v,Current:%v,Root:%s,BasePath:%s,PathInfo:%s,DirInfo:%s,Files:%v,Dirs:%v,ImageCount:%v,FullTitle:%s, PageTitle:%s,ActualPath:%s,Mp4Path:%s,BaseFilename:%s,FileIndex:%d,PrevSeven:%s,NextSeven:%s,CaptionHtml:%s,CaptionMap:%v}`,
		t.AppConfig, t.AlbumConfig, t.AlbumConfig, t.Current, t.Root, t.BasePath, t.PathInfo, t.DirInfo, t.Files, t.Dirs, t.ImageCount, t.FullTitle, t.PageTitle, t.ActualPath, t.Mp4Path, t.BaseFilename, t.FileIndex, t.PrevSeven, t.NextSeven, t.CaptionHtml, t.CaptionMap)
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
	if b.BodyArgs != "" {
		a.BodyArgs = b.BodyArgs
	}

	if b.VideoThumbnailSize != "" {
		a.VideoThumbnailSize = b.VideoThumbnailSize
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
