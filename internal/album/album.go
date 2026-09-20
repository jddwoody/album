package album

/*
   Copyright 1998-2021 James D Woodgate.  All rights reserved.
   It may be used and modified freely, but I do request that this copyright
   notice remain attached to the file.  You may modify this module as you
   wish, but if you redistribute a modified version, please attach a note
   listing the modifications you have made.
*/

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/disintegration/imaging"
	"gopkg.in/yaml.v2"
)

func (a Album) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'; script-src 'none'; object-src 'none'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")

	switch req.Method {
	case "GET":
		a.handleGet(w, req)
	default:
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a Album) handleGet(w http.ResponseWriter, req *http.Request) {
	url := req.URL
	path := url.Path
	fmt.Printf("url.Path:%s\n", path)
	var tmpl *template.Template
	appConfig, err := LoadAppConfigFile()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	albumsConfig, err := LoadAlbumsConfigFile(appConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmplSource := TemplateSource{AppConfig: appConfig, AlbumsConfig: albumsConfig}

	paths := strings.SplitN(path[1:], "/", 3)
	if len(paths) < 3 {
		// It should always be at least 2, so show page with available albums
		tmpl = template.Must(template.New("base").Parse(`<HTML>
  <HEADER><TITLE>Available Albums</TITLE></HEADER>
	<BODY {{ .SafeBodyArgs }}>
    <H3>Available Albums</H3>
	{{ range .SortedAlbumTitles }}
	<a href="/{{ .Key }}/albums/">{{ .Title }}</a><br>
	{{ end }}
  </BODY>
</HTML>
`))

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, albumsConfig); err != nil {
			fmt.Printf("Error rendering album index: %v\n", err)
		}
		return
	}

	if paths[1] == "thumbs" {
		// must be a thumbnail, files only
		a.handleThumbnail(w, req, appConfig, albumsConfig, paths[0], paths[2])
		return
	}

	if paths[1] != "albums" {
		// Invalid path
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer func() {
		if tmpl == nil {
			return
		}

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, tmplSource); err != nil {
			fmt.Printf("Error rendering album page: %v\n", err)
		}
	}()

	// Paths[0] should match an album id, Paths[1] should be either albums or thumbs
	tmplSource.Root = path
	tmplSource.BasePath = paths[0]
	tmplSource.PathInfo = paths[2]
	tmplSource.PathInfo = strings.TrimSuffix(tmplSource.PathInfo, "/")
	if IsViewableFile(tmplSource.PathInfo) {
		tmplSource.DirInfo = filepath.Dir(tmplSource.Root)
	} else {
		tmplSource.DirInfo = tmplSource.Root
	}
	var ok bool
	albumConfig, ok := albumsConfig.Albums[paths[0]]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	tmplSource.AlbumConfig = albumConfig
	tmplSource.Current = albumsConfig.Default
	Merge(&tmplSource.Current, &albumConfig.Config)
	baseDir := filepath.Join(appConfig.AlbumsDir, tmplSource.AlbumConfig.AlbumDir)
	albumPathInfo, err := safeJoin(baseDir, tmplSource.PathInfo)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(albumPathInfo)
	if err != nil {
		filename := filepath.Base("/" + tmplSource.PathInfo)
		// if the filename starts with 640x480_, 800x600_ or 1024x768_, set imgLink to
		// thumbs and let the normal handler take care of it
		if strings.HasPrefix(filename, "640x480_") || strings.HasPrefix(filename, "800x600_") || strings.HasPrefix(filename, "1024x768_") {
			a.handleThumbnail(w, req, appConfig, albumsConfig, paths[0], paths[2])
			return
		} else {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	}

	playVideo := req.URL.Query().Get("playvideo")
	if playVideo != "" && stat.Mode().IsRegular() {
		tmplSource.BaseFilename = filepath.Base(tmplSource.PathInfo)
		videoDir, err := safeJoin(baseDir, filepath.Dir(tmplSource.PathInfo))
		if err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		dirEntries, err := os.ReadDir(videoDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, dirEntry := range dirEntries {
			if IsVideoFile(dirEntry.Name()) {
				tmplSource.Files = append(tmplSource.Files, dirEntry)
			}
		}

		for idx, dirEntry := range tmplSource.Files {
			if dirEntry.Name() == tmplSource.BaseFilename {
				tmplSource.FileIndex = idx
			}
		}
		lastIndex := len(tmplSource.Files) - 1

		if lastIndex > 7 {
			if tmplSource.FileIndex > 3 {
				less := tmplSource.FileIndex - 3
				if less > 7 {
					less = 7
				}
				move := 0
				if tmplSource.FileIndex > lastIndex-3 {
					move = lastIndex - tmplSource.FileIndex
				}
				prevName := tmplSource.Files[tmplSource.FileIndex-less-move].Name()
				tmplSource.PrevSeven = template.HTML(fmt.Sprintf(`<TD ALIGN="left"><A HREF="%s?playvideo=1">&lt;Prev %d&lt;</A></TD>`,
					escapeTemplateHTML(fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), prevName)), less))
			}

			if tmplSource.FileIndex < lastIndex-3 {
				more := lastIndex - 3 - tmplSource.FileIndex
				if more > 7 {
					more = 7
				}
				move := 0
				if tmplSource.FileIndex < 3 {
					move = 3
				}
				nextName := tmplSource.Files[tmplSource.FileIndex+more+move].Name()
				currentBase := filepath.Base(tmplSource.Root)
				tmplSource.NextSeven = template.HTML(fmt.Sprintf(`<TD ALIGN="right"><A HREF="%s?playvideo=1">&gt;Next %d&gt;</A></TD>`,
					escapeTemplateHTML(fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), fixNextName(currentBase, nextName))), more))
			}
		}

		lowerIndex := 0
		extra := 0
		upperIndex := lastIndex
		if len(tmplSource.Files) > 7 {
			if tmplSource.FileIndex > 3 {
				lowerIndex = tmplSource.FileIndex - 3
			} else {
				extra = 3 - tmplSource.FileIndex
			}
			if lastIndex-tmplSource.FileIndex > 3 {
				upperIndex = tmplSource.FileIndex + 3 + extra
			} else {
				lowerIndex -= 3 - (lastIndex - tmplSource.FileIndex)
			}
		}
		currentBase := filepath.Base(tmplSource.Root)
		thumbnailLinks := ""
		for i := lowerIndex; i <= upperIndex; i++ {
			filename := tmplSource.Files[i].Name()
			tnImgSrc := fmt.Sprintf("/%s/thumbs/%s/tn__%s", tmplSource.BasePath, filepath.Dir(tmplSource.PathInfo), ChangeExtension(filename, "png"))
			extraTd := ""
			if i == tmplSource.FileIndex {
				extraTd = ` bgcolor="blue"`
			}

			thumbnailLinks += fmt.Sprintf(`<TD%s><A HREF="%s?playvideo=1"><IMG SRC="%s" height="60" title="Click to Play Video"></A></TD>`, extraTd, escapeTemplateHTML(fixNextName(currentBase, filename)), escapeTemplateHTML(tnImgSrc))
		}

		if CanHtmlPlay(tmplSource.BaseFilename) {
			tmplSource.ActualPath = tmplSource.Root
		} else {
			tmplSource.ActualPath = fmt.Sprintf("/%s/thumbs/%s", tmplSource.BasePath, ChangeExtension(tmplSource.PathInfo, "webm"))
			tmplSource.Mp4Path = fmt.Sprintf("/%s/thumbs/%s", tmplSource.BasePath, ChangeExtension(tmplSource.PathInfo, "mp4"))
			thumbDir := filepath.Join(appConfig.AlbumsDir, tmplSource.AlbumConfig.ThumbDir)
			mp4ActualFile, err := safeJoin(thumbDir, ChangeExtension(tmplSource.PathInfo, "mp4"))
			if err != nil {
				http.Error(w, "invalid path", http.StatusBadRequest)
				return
			}
			fmt.Printf("mp4ActualFile:%s\n", mp4ActualFile)
			if _, err := os.Stat(mp4ActualFile); errors.Is(err, os.ErrNotExist) {
				source := albumPathInfo
				fmt.Printf("Need to generate mp4:%s from %s\n", mp4ActualFile, source)
				if err := os.MkdirAll(filepath.Dir(mp4ActualFile), 0755); err != nil {
					http.Error(w, "unable to create thumbnail directory", http.StatusInternalServerError)
					return
				}
				if err := ConvertVideoFile(source, mp4ActualFile); err != nil {
					http.Error(w, "unable to convert video", http.StatusInternalServerError)
					return
				}
			}
		}
		tmpl = template.Must(template.New("base").Parse(pictureDirHeader(false) + fmt.Sprintf(
			`            <center><TABLE BORDER="0" CELLPADDING="4" CELLSPACING="0"><TR>{{ .PrevSeven }}%s{{ .NextSeven }}</TR></TABLE>
		<HR>
            <TR>
			<CENTER>
			  <video style="max-width: 1024px" controls>
			    <source src="{{ $.ActualPath }}" />
			    <source src="{{ $.Mp4Path }}" />
			  </video>
			</CENTER>
			<CENTER>{{ $.MakePicTitle $.BaseFilename }}</CENTER><HR>
			</TR>
`, thumbnailLinks) + pictureDirFooter()))
		return
	}

	slideShow := req.URL.Query().Get("slide_show")
	if slideShow != "" && stat.Mode().IsRegular() {
		if IsImageFile(tmplSource.PathInfo) {
			tmplSource.ActualPath = fmt.Sprintf("/%s/thumbs/%s/%s", tmplSource.BasePath, filepath.Dir(tmplSource.PathInfo), changeSize(slideShow, filepath.Base(tmplSource.PathInfo)))
			tmplSource.BaseFilename = filepath.Base("/" + tmplSource.PathInfo)
		} else {
			// Can't do a slide show of videos
			http.Error(w, "Can't do slide show of videos", http.StatusNotFound)
			return
		}
	}
	if tmplSource.ActualPath == "" && stat.Mode().IsRegular() {
		http.ServeFile(w, req, albumPathInfo)
		return
	}

	albumDir := albumPathInfo
	if tmplSource.ActualPath != "" {
		albumDir = filepath.Dir(albumPathInfo)
	}
	dirEntries, err := os.ReadDir(albumDir)
	if err != nil {
		http.Error(w, "unable to read album directory", http.StatusInternalServerError)
		return
	}

	var captionFile *CaptionFile
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			if !strings.HasPrefix(dirEntry.Name(), ".") {
				tmplSource.Dirs = append(tmplSource.Dirs, dirEntry)
			}
		} else {
			if dirEntry.Name() == "caption.txt" {
				in, err := os.Open(filepath.Join(albumDir, dirEntry.Name()))
				if err == nil {
					captionFile = NewCaptionFile(in)
					_ = in.Close()
				}
			} else if dirEntry.Name() == "config.yaml" {
				fmt.Println("Found config.yaml")
				in, err := os.Open(filepath.Join(albumDir, dirEntry.Name()))
				if err == nil {
					decoder := yaml.NewDecoder(in)
					var dirConfig Config
					err = decoder.Decode(&dirConfig)
					_ = in.Close()
					if err == nil {
						fmt.Printf("new dirconfig: %v\n", dirConfig)
						Merge(&tmplSource.Current, &dirConfig)
					} else {
						fmt.Printf("Error decoding config file: %v\n", err)
					}
				} else {
					fmt.Printf("Error opening config file: %v\n", err)
				}
			} else {
				if !strings.HasPrefix(dirEntry.Name(), ".") && IsViewableFile(dirEntry.Name()) {
					tmplSource.Files = append(tmplSource.Files, dirEntry)
					if IsImageFile(dirEntry.Name()) {
						tmplSource.ImageCount += 1
					}

					// A video file that isn't html viewable needs to be converted
					if VideoNeedsConversion(dirEntry.Name()) {
						originalFilename := fmt.Sprintf("%s/%s", albumDir, dirEntry.Name())
						thumbRoot := filepath.Join(appConfig.AlbumsDir, tmplSource.AlbumConfig.ThumbDir)
						convertedFilename, err := safeJoin(thumbRoot, filepath.Join(tmplSource.PathInfo, ChangeExtension(dirEntry.Name(), "webm")))
						if err != nil {
							continue
						}
						if _, err := os.Stat(convertedFilename); errors.Is(err, os.ErrNotExist) {
							workingMu.Lock()
							alreadyWorking := workingMap[originalFilename]
							if !alreadyWorking {
								workingMap[originalFilename] = true
							}
							workingMu.Unlock()
							if !alreadyWorking {
								fmt.Printf("Converting %s to %s\n", originalFilename, convertedFilename)
								pool.Submit(func() {
									defer func() {
										workingMu.Lock()
										delete(workingMap, originalFilename)
										workingMu.Unlock()
									}()
									if err := ConvertVideoFile(originalFilename, convertedFilename); err != nil {
										fmt.Printf("Error converting %s: %v\n", originalFilename, err)
									}
								})
							}
						}
					}
				}
			}
		}
	}

	if captionFile != nil {
		tmplSource.CaptionHtml = template.HTML(captionFile.Html)
		tmplSource.CaptionMap = captionFile.CaptionMap
	}

	if tmplSource.Current.ReverseDirs {
		sort.Slice(tmplSource.Dirs, func(i, j int) bool {
			return tmplSource.Dirs[i].Name() > tmplSource.Dirs[j].Name()
		})
	} else {
		sort.Slice(tmplSource.Dirs, func(i, j int) bool {
			return tmplSource.Dirs[i].Name() < tmplSource.Dirs[j].Name()
		})
	}

	if len(tmplSource.Files) == 0 {
		// No images, just show directories
		paths := strings.Split(tmplSource.PathInfo, "/")
		for idx, ele := range paths {
			paths[idx] = beautify(ele)
		}
		tmplSource.FullTitle = strings.Join(paths, " - ")
		tmpl = a.generateDirs()
		return
	}

	allFullImages := req.URL.Query().Get("all_full_images")
	if allFullImages != "" {
		root := "albums"
		prefix := ""
		if allFullImages != "full" {
			root = "thumbs"
			prefix = prefixMap[allFullImages]
		}
		tmplSource.PageTitle = strings.ReplaceAll(beautify(tmplSource.PathInfo), "/", " - ")
		tmplSource.FullTitle = tmplSource.PageTitle
		tmpl = template.Must(template.New("base").Parse(pictureDirHeader(true) +
			`           <TR>
			{{ range $index,$ele := .GetImageFiles }}
			<CENTER><IMG SRC="/{{ $.BasePath }}/` + root + `/{{ $.PathInfo }}/` + prefix + `{{ $ele.Name }}" ALT="{{ $ele.Name }}"></CENTER><HR>
			<CENTER>{{ $.MakePicTitle $ele.Name }}</CENTER><HR>
			{{ end }}
			</TR>
` + pictureDirFooter()))
		return
	}

	tmplSource.PageTitle = beautify(filepath.Base(tmplSource.PathInfo))
	dir := filepath.Dir(tmplSource.PathInfo)
	if dir != "" && dir != "." {
		paths := strings.Split(dir, "/")
		for idx, ele := range paths {
			paths[idx] = beautify(ele)
		}
		if tmplSource.ActualPath == "" {
			// Must be dir
			tmplSource.FullTitle = strings.Join(paths, " - ") + " - " + tmplSource.PageTitle
		} else {
			tmplSource.FullTitle = strings.Join(paths, " - ")
		}
	} else {
		tmplSource.FullTitle = tmplSource.PageTitle
	}

	tmplText := ""
	imageFiles := GetImageFiles(tmplSource.Files)
	if tmplSource.ActualPath == "" {
		if slideShow != "" && len(imageFiles) > 0 {
			// If there isn't a filename and slideShow is enabled, just call the first picture
			http.Redirect(w, req, fmt.Sprintf("%s/%s?slide_show=%s", tmplSource.Root, tmplSource.Files[0].Name(), slideShow), http.StatusTemporaryRedirect)
			return
		}
		if tmplSource.Current.NumberOfColumns > 0 {
			tmplSource.NumberOfColumns = tmplSource.Current.NumberOfColumns
		} else {
			tmplSource.NumberOfColumns = tmplSource.Current.GetDefaultBrowserWidth() / tmplSource.Current.GetThumbnailWidth()
		}
		tmplText = `           <TR>
		{{ range $index,$ele := .Files }}
			{{ if $.NeedNewRow $index}}
		</TR>
		<TR>  
			{{ end }}
		  <TD ALIGN="center">
			<TABLE BORDER={{ $.Current.InsideTableBorder }}>
			{{ if $.IsImageFile $ele.Name }}
			  <TR>
				<TD ALIGN="center"><A HREF="{{ $ele.Name }}"><IMG SRC="/{{ $.BasePath }}/thumbs/{{ $.PathInfo }}/tn__{{ $ele.Name }}" ALT="{{ $ele.Name }}"></A></TD>
			  </TR>
			  <TR>
				<TD ALIGN="center"><A HREF="640x480_{{ $ele.Name }}">Sm</A> <A HREF="800x600_{{ $ele.Name }}">Med</A> </A><A HREF="1024x768_{{ $ele.Name }}">Lg</A><BR>
				{{ $.MakePicTitle $ele.Name }}
				</TD>
			  </TR>
			{{ else }}
			  <TR>
				<TD ALIGN="center"><A HREF="{{ $ele.Name }}?playvideo=1"><IMG SRC="/{{ $.BasePath }}/thumbs/{{ $.PathInfo }}/tn__{{ $.AsPngFilename $ele.Name }}" ALT="{{ $.AsPngFilename $ele.Name }}" title="Click to Play Video"></A></TD>
			  </TR>
			  <TR>
				<TD ALIGN="center">{{ $.MakePicTitle $ele.Name }}</TD>
			  </TR>
		    {{ end }}
			</TABLE>
		  </TD>
		{{ end }}
		</TR>
`
	} else {
		for idx, dirEntry := range imageFiles {
			if dirEntry.Name() == tmplSource.BaseFilename {
				tmplSource.FileIndex = idx
			}
		}
		lastIndex := len(imageFiles) - 1
		if lastIndex > 7 {
			if tmplSource.FileIndex > 3 {
				less := tmplSource.FileIndex - 3
				if less > 7 {
					less = 7
				}
				move := 0
				if tmplSource.FileIndex > lastIndex-3 {
					move = lastIndex - tmplSource.FileIndex
				}
				prevName := imageFiles[tmplSource.FileIndex-less-move].Name()
				currentBase := filepath.Base(tmplSource.Root)
				if strings.HasPrefix(currentBase, "640x480_") {
					prevName = "640x480_" + prevName
				} else if strings.HasPrefix(currentBase, "800x600_") {
					prevName = "800x600_" + prevName
				} else if strings.HasPrefix(currentBase, "1024x768_") {
					prevName = "1024x768_" + prevName
				}

				tmplSource.PrevSeven = template.HTML(fmt.Sprintf(`<TD ALIGN="left"><A HREF="%s">&lt;Prev %d&lt;</A></TD>`,
					escapeTemplateHTML(fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), prevName)), less))
			}

			if tmplSource.FileIndex < lastIndex-3 {
				more := lastIndex - 3 - tmplSource.FileIndex
				if more > 7 {
					more = 7
				}
				move := 0
				if tmplSource.FileIndex < 3 {
					move = 3
				}
				nextName := imageFiles[tmplSource.FileIndex+more+move].Name()
				currentBase := filepath.Base(tmplSource.Root)
				tmplSource.NextSeven = template.HTML(fmt.Sprintf(`<TD ALIGN="right"><A HREF="%s">&gt;Next %d&gt;</A></TD>`,
					escapeTemplateHTML(fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), fixNextName(currentBase, nextName))), more))
			}
		}

		lowerIndex := 0
		extra := 0
		upperIndex := lastIndex
		if len(imageFiles) > 7 {
			if tmplSource.FileIndex > 3 {
				lowerIndex = tmplSource.FileIndex - 3
			} else {
				extra = 3 - tmplSource.FileIndex
			}
			if lastIndex-tmplSource.FileIndex > 3 {
				upperIndex = tmplSource.FileIndex + 3 + extra
			} else {
				lowerIndex -= 3 - (lastIndex - tmplSource.FileIndex)
			}
		}
		currentBase := filepath.Base(tmplSource.Root)
		thumbnailLinks := ""
		for i := lowerIndex; i <= upperIndex; i++ {
			filename := imageFiles[i].Name()
			tnImgSrc := fmt.Sprintf("/%s/thumbs/%s/tn__%s", tmplSource.BasePath, filepath.Dir(tmplSource.PathInfo), filename)
			extraTd := ""
			if i == tmplSource.FileIndex {
				extraTd = ` bgcolor="blue"`
			}

			thumbnailLinks += fmt.Sprintf(`<TD%s><A HREF="%s"><IMG SRC="%s" height="60"></A></TD>`, extraTd, fixNextName(currentBase, filename), tnImgSrc)
		}
		tmplText = fmt.Sprintf(`
		<center><TABLE BORDER="0" CELLPADDING="4" CELLSPACING="0"><TR>{{ .PrevSeven }}%s{{ .NextSeven }}</TR></TABLE>
		<HR>
		<CENTER><A HREF="{{ .BaseFilename }}" BORDER="0"><IMG SRC="{{ .ActualPath }}" ALT="{{ .PathInfo }}"></A>
<HR>
<H3>{{ $.MakePicTitle .BaseFilename}}</H3></CENTER>
<HR>`, thumbnailLinks)

		// If it's a slideshow show, set up a refresh
		if slideShow != "" && tmplSource.FileIndex < lastIndex {
			w.Header().Set("Refresh", fmt.Sprintf("%d; URL=%s?slide_show=%s", tmplSource.Current.SlideShowDelay, imageFiles[tmplSource.FileIndex+1].Name(), slideShow))
		}

	}

	// We have files so go ahead and build a table of thumbnails
	tmpl = template.Must(template.New("base").Parse(pictureDirHeader(true) + tmplText + pictureDirFooter()))
}

func (a Album) generateDirs() *template.Template {
	return template.Must(template.New("base").Parse(`
	<HTML>
		<HEADER><TITLE>{{ .AlbumConfig.AlbumTitle }}</TITLE></HEADER>
		<BODY {{ .AlbumsConfig.SafeBodyArgs }}>
			<H3>{{ .AlbumConfig.AlbumTitle }}</H3>
			{{ .FullTitle }}
			{{ range .Dirs }}
			<dl>
			  {{ $.HandleDirs . "" 0}}
			</dl>
			{{ end }}
		</BODY>
	</HTML>
	`))
}

func (a Album) handleThumbnail(w http.ResponseWriter, req *http.Request, appConfig *AppConfig, albumsConfig *AlbumsConfig, albumName, pathInfo string) {
	config := albumsConfig.Default
	albumConfig, ok := albumsConfig.Albums[albumName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	Merge(&config, &albumConfig.Config)
	thumbDir := filepath.Join(appConfig.AlbumsDir, albumConfig.ThumbDir)
	fullFilename, err := safeJoin(thumbDir, pathInfo)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	_, err = os.Stat(fullFilename)

	// err just means we need to create it
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(fullFilename), 0755); err != nil {
			http.Error(w, "unable to create thumbnail directory", http.StatusInternalServerError)
			return
		}

		fullAlbumDir := filepath.Join(appConfig.AlbumsDir, albumConfig.AlbumDir)
		sourceFilename, err := safeJoin(fullAlbumDir, cleanTn(pathInfo))
		if err != nil {
			http.Error(w, "invalid source path", http.StatusBadRequest)
			return
		}
		if IsImageFile(pathInfo) {
			img, err := imaging.Open(sourceFilename, imaging.AutoOrientation(true))
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			filename := path.Base(pathInfo)
			width := int(config.GetThumbnailWidth())
			if strings.HasPrefix(filename, "640") {
				width = 640
			} else if strings.HasPrefix(filename, "800") {
				width = 800
			} else if strings.HasPrefix(filename, "1024") {
				width = 1024
			}
			dstImage := imaging.Resize(img, width, 0, imaging.Box)
			if err := imaging.Save(dstImage, fullFilename); err != nil {
				http.Error(w, "unable to save thumbnail", http.StatusInternalServerError)
				return
			}
		} else {
			// Must be video, need to figure out the original filename and save a frame
			clean := cleanTn(pathInfo)
			prefix := strings.TrimSuffix(clean, filepath.Ext(clean))
			sourcePrefix, err := safeJoin(fullAlbumDir, prefix)
			if err != nil {
				http.Error(w, "invalid source path", http.StatusBadRequest)
				return
			}
			sourceGlob := sourcePrefix + "*"
			glob, err := filepath.Glob(sourceGlob)
			if err != nil {
				fmt.Printf("Glob error using %s, err: %s\n", prefix, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if len(glob) == 0 {
				fmt.Printf("No match for %s\n", prefix)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			err = GenerateVideoThumbnail(glob[0], config.GetVideoThumbnailSize(), fullFilename)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
	} else if err != nil {
		http.Error(w, "unable to inspect thumbnail", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, req, fullFilename)
}

func (a Album) Footer() string {
	fmt.Printf("  *****     Normal footer     ***********")
	return `
	<center>
		Slide Show: <a href="?slide_show=sm">small</a> | <a href="?slide_show=med">medium</a> | <a href="?slide_show=lg">large</a> | <a href="?slide_show=full">full sized</a>
	</center>
	<br>
	<center>
		All Images: <a href="?all_full_images=sm">small</a> | '<a href="?all_full_images=med">medium</a> | <a href="?all_full_images=lg">large</a> | <a href="?all_full_images=full">full sized</a>
	</center>
`
}

func (a AlbumsConfig) SortedAlbumTitles() []AlbumTitle {
	titles := make([]AlbumTitle, 0)
	for key, value := range a.Albums {
		titles = append(titles, AlbumTitle{
			Key:   key,
			Title: value.AlbumTitle,
		})
	}

	sort.Slice(titles, func(i, j int) bool { return titles[i].Title < titles[j].Title })
	return titles
}

func (t TemplateSource) NeedNewRow(index int) bool {
	return t.NumberOfColumns > 0 && index > 0 && index%t.NumberOfColumns == 0
}

func (t TemplateSource) IsImageFile(filename string) bool {
	return IsImageFile(filename)
}

func (t TemplateSource) GetImageFiles() []os.DirEntry {
	return GetImageFiles(t.Files)
}

func (t TemplateSource) AsPngFilename(filename string) string {
	return ChangeExtension(filename, "png")
}

func (t TemplateSource) HandleDirs(f os.DirEntry, subdir string, depth int) template.HTML {
	if depth >= 64 {
		return template.HTML("")
	}
	// Check directory to see if there are sub-directories
	newSubDir := f.Name()
	if subdir != "" {
		newSubDir = subdir + "/" + f.Name()
	}
	dir := filepath.Join(t.AppConfig.AlbumsDir, t.AlbumConfig.AlbumDir)
	var err error
	if t.PathInfo == "" {
		dir, err = safeJoin(dir, newSubDir)
	} else {
		dir, err = safeJoin(dir, filepath.Join(t.PathInfo, newSubDir))
	}
	if err != nil {
		return template.HTML("")
	}

	children := template.HTML("")
	dirEntries, err := os.ReadDir(dir)
	if err == nil {
		if t.Current.ReverseDirs {
			sort.Slice(dirEntries, func(i, j int) bool {
				return dirEntries[i].Name() > dirEntries[j].Name()
			})
		} else {
			sort.Slice(dirEntries, func(i, j int) bool {
				return dirEntries[i].Name() < dirEntries[j].Name()
			})
		}

		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				children += t.HandleDirs(dirEntry, newSubDir, depth+1)
			}
		}
		if children != "" {
			children = "\n                <dd><dl>" + children + "</dl></dd>\n"
		}

	}

	return template.HTML(fmt.Sprintf(`<dt><a href="%s%s/">%s</a></dt>`,
		escapeTemplateHTML(t.Root), escapeTemplateHTML(newSubDir), escapeTemplateHTML(beautify(f.Name()))) + string(children))
}

func (t TemplateSource) MakePicTitle(s string) string {
	caption, ok := t.CaptionMap[s]
	if ok {
		return caption
	}
	s = strings.Split(s, ".")[0]
	return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "-", " ")
}

func beautify(s string) string {
	re := regexp.MustCompile(`\d+\((.*)\)`)
	matches := re.FindSubmatch([]byte(s))
	if matches != nil {
		s = string(matches[1])
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "-", " ")
}

func cleanTn(filename string) string {
	// The thumbnail will be named something like /a/b/c/tn__filename.jpg or /a/b/c/800x600filename.jpg
	// need to get rid of the tn__ or 800x600 to get to the actual filename
	file := path.Base(filename)
	if strings.HasPrefix(file, "tn__") {
		return fmt.Sprintf("%s/%s", path.Dir(filename), file[4:])
	}

	if strings.HasPrefix(file, "800x600_") || strings.HasPrefix(file, "640x480_") {
		return fmt.Sprintf("%s/%s", path.Dir(filename), file[8:])
	}

	if strings.HasPrefix(file, "1024x768_") {
		return fmt.Sprintf("%s/%s", path.Dir(filename), file[9:])
	}
	return filename
}

func pictureDirHeader(includeExtraTitle bool) string {
	extraTitle := ""
	height := "125"
	if includeExtraTitle {
		extraTitle = `<CENTER>{{ .AlbumConfig.AlbumTitle }} - {{ .FullTitle }}</CENTER>`
		height = "150"
	}
	return `
	<HTML>
		<HEADER><TITLE>{{ .PageTitle }}</TITLE></HEADER>
		<BODY {{ .Current.SafeBodyArgs }}>` + extraTitle +
		`<HR />
		<CENTER>
		  <div style="overflow: auto; height: calc(100vh - ` + height + `px)">
		  {{ .CaptionHtml }}
		  <TABLE BORDER={{ .Current.OutsideTableBorder }}>	`
}

func escapeTemplateHTML(value string) string {
	return strings.NewReplacer("{", "&#123;", "}", "&#125;").Replace(template.HTMLEscapeString(value))
}

func pictureDirFooter() string {
	return `
	  </div>
	  </TABLE>
	</CENTER>
	<HR>
	<CENTER>{{ if gt .ImageCount 0 }}Slide Show: <a href="?slide_show=sm">small</a> | <a href="?slide_show=med">medium</a> | <a href="?slide_show=lg">large</a> | <a href="?slide_show=full">full sized</a><br>
			All Images: <a href="{{ .DirInfo }}?all_full_images=sm">small</a> | <a href="{{ .DirInfo }}?all_full_images=med">medium</a> | <a href="{{ .DirInfo }}?all_full_images=lg">large</a> | <a href="{{ .DirInfo }}?all_full_images=full">full sized</a><br>
			<a href="./">Back to thumbnails</a><br>{{ end }}
			<a href="/{{ .BasePath }}/albums/">Back to {{ .AlbumConfig.AlbumTitle }}</a>
	</CENTER>
</BODY>
</HTML>`
}

func fixNextName(currentBase, nextName string) string {
	if strings.HasPrefix(currentBase, "640x480_") {
		return "640x480_" + nextName
	}
	if strings.HasPrefix(currentBase, "800x600_") {
		return "800x600_" + nextName
	}
	if strings.HasPrefix(currentBase, "1024x768_") {
		return "1024x768_" + nextName
	}
	return nextName
}

func changeSize(name, filename string) string {
	switch name {
	case "sm":
		return "640x480_" + filename
	case "med":
		return "800x600_" + filename
	case "lg":
		return "1024x768_" + filename
	}

	return filename
}

func LoadAppConfigFile() (*AppConfig, error) {
	in, err := os.Open(APP_CONFIG_FILENAME)
	if err != nil {
		return nil, err
	}

	defer in.Close()
	decoder := yaml.NewDecoder(in)
	var app AppConfig
	err = decoder.Decode(&app)
	if err != nil {
		return nil, err
	}

	return &app, nil
}

func LoadAlbumsConfigFile(appConfig *AppConfig) (*AlbumsConfig, error) {
	in, err := os.Open(filepath.Join(appConfig.AlbumsDir, ALBUMS_CONFIG_FILENAME))
	if err != nil {
		return nil, err
	}

	defer in.Close()
	decoder := yaml.NewDecoder(in)
	var albumsConfig AlbumsConfig
	err = decoder.Decode(&albumsConfig)
	if err != nil {
		return nil, err
	}

	return &albumsConfig, nil
}
