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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/disintegration/imaging"
	"gopkg.in/yaml.v2"
)

func (a Album) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		a.handleGet(w, req)
	}
}

func (a Album) handleGet(w http.ResponseWriter, req *http.Request) {
	url := req.URL
	path := url.Path
	fmt.Printf("url.Path:%s\n", path)
	var tmpl *template.Template
	app, err := LoadConfigFile()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmplSource := TemplateSource{App: app}

	paths := strings.SplitN(path[1:], "/", 3)
	if len(paths) < 3 {
		// It should always be at least 2, so show page with available albums
		tmpl = template.Must(template.New("base").Parse(`<HTML>
  <HEADER><TITLE>Available Albums</TITLE></HEADER>
  <BODY {{ .BodyArgs }}>
    <H3>Available Albums</H3>
	{{ range .SortedAlbumTitles }}
	<a href="/{{ .Key }}/albums/">{{ .Title }}</a><br>
	{{ end }}
  </BODY>
</HTML>
`))

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		tmpl.Execute(w, app)
		return
	}

	if paths[1] == "thumbs" {
		// must be a thumbnail, files only
		a.handleThumbnail(w, req, app, paths[0], paths[2])
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
		w.WriteHeader(http.StatusOK)
		tmpl.Execute(w, tmplSource)
	}()

	// Paths[0] should match an album id, Paths[1] should be either albums or thumbs
	tmplSource.Root = path
	tmplSource.BasePath = paths[0]
	tmplSource.PathInfo = paths[2]
	tmplSource.PathInfo = strings.TrimSuffix(tmplSource.PathInfo, "/")
	var ok bool
	tmplSource.Current = app.Default
	albumConfig, ok := app.Albums[paths[0]]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	Merge(&tmplSource.Current, &albumConfig)
	baseDir := tmplSource.Current.AlbumDir
	albumPathInfo := fmt.Sprintf("%s/%s", baseDir, tmplSource.PathInfo)

	stat, err := os.Stat(albumPathInfo)
	if err != nil {
		filename := filepath.Base("/" + tmplSource.PathInfo)
		// if the filename starts with 640x480_, 800x600_ or 1024x768_, set imgLink to
		// thumbs and let the normal handler take care of it
		if strings.HasPrefix(filename, "640x480_") || strings.HasPrefix(filename, "800x600_") || strings.HasPrefix(filename, "1024x768_") {
			tmplSource.ActualPath = fmt.Sprintf("/%s/thumbs/%s", tmplSource.BasePath, tmplSource.PathInfo)
			tmplSource.BaseFilename = filepath.Base(cleanTn(tmplSource.ActualPath))
		} else {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}

	playVideo := req.URL.Query().Get("playvideo")
	if playVideo != "" && stat.Mode().IsRegular() {
		tmplSource.BaseFilename = filepath.Base(tmplSource.PathInfo)
		videoDir := fmt.Sprintf("%s/%s", baseDir, filepath.Dir(tmplSource.PathInfo))
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
				tmplSource.PrevSeven = fmt.Sprintf(`<TD ALIGN="left"><A HREF="%s?playvideo=1")>&lt;Prev %d&lt;</A></TD>`,
					fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), prevName), less)
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
				tmplSource.NextSeven = fmt.Sprintf(`<TD ALIGN="right"><A HREF="%s?playvideo=1">&gt;Next %d&gt;</A></TD>`,
					fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), fixNextName(currentBase, nextName)), more)
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

			thumbnailLinks += fmt.Sprintf(`<TD%s><A HREF="%s?playvideo=1"><IMG SRC="%s" height="60" title="Click to Play Video"></A></TD>`, extraTd, fixNextName(currentBase, filename), tnImgSrc)
		}

		if CanHtmlPlay(tmplSource.BaseFilename) {
			tmplSource.ActualPath = tmplSource.Root
		} else {
			tmplSource.ActualPath = fmt.Sprintf("/%s/thumbs/%s", tmplSource.BasePath, ChangeExtension(tmplSource.PathInfo, "webm"))
		}
		tmpl = template.Must(template.New("base").Parse(pictureDirHeader(false) + fmt.Sprintf(
			`            <center><TABLE BORDER="0" CELLPADDING="4" CELLSPACING="0"><TR>{{ .PrevSeven }}%s{{ .NextSeven }}</TR></TABLE>
		<HR>
            <TR>
			<CENTER>
			  <video src="{{ $.ActualPath }}" controls />
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
				in, err := os.Open(fmt.Sprintf("%s/%s", albumDir, dirEntry.Name()))
				if err == nil {
					defer in.Close()
					captionFile = NewCaptionFile(in)
				}
			} else if dirEntry.Name() == "config.yaml" {
				in, err := os.Open(fmt.Sprintf("%s/%s", albumDir, dirEntry.Name()))
				if err == nil {
					defer in.Close()
					decoder := yaml.NewDecoder(in)
					var dirConfig Config
					err = decoder.Decode(&dirConfig)
					if err == nil {
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

					// A video file that isn't html viewable needs to be converted
					if VideoNeedsConversion(dirEntry.Name()) {
						originalFilename := fmt.Sprintf("%s/%s", albumDir, dirEntry.Name())
						thumbActualDir := fmt.Sprintf("%s/%s", tmplSource.Current.ThumbDir, tmplSource.PathInfo)
						convertedFilename := fmt.Sprintf("%s/%s", thumbActualDir, ChangeExtension(dirEntry.Name(), "webm"))
						if _, err := os.Stat(convertedFilename); errors.Is(err, os.ErrNotExist) {
							if !workingMap[originalFilename] {
								fmt.Printf("Converting %s to %s\n", originalFilename, convertedFilename)
								workingMap[originalFilename] = true
								pool.Submit(func() {
									ConvertVideoFile(originalFilename, convertedFilename)
									//delete(workingMap, originalFilename)
								})
							}
						}
					}
				}
			}
		}
	}

	if captionFile != nil {
		tmplSource.CaptionHtml = captionFile.Html
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
		tmpl = template.Must(template.New("base").Parse(pictureDirHeader(true) +
			`           <TR>
			{{ range $index,$ele := .Files }}
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
	fmt.Printf("ActualPath:%s, slideshow:%s, len(files):%d\n", tmplSource.ActualPath, slideShow, len(tmplSource.Files))
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

				tmplSource.PrevSeven = fmt.Sprintf(`<TD ALIGN="left"><A HREF="%s")>&lt;Prev %d&lt;</A></TD>`,
					fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), prevName), less)
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
				tmplSource.NextSeven = fmt.Sprintf(`<TD ALIGN="right"><A HREF="%s">&gt;Next %d&gt;</A></TD>`,
					fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), fixNextName(currentBase, nextName)), more)
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
		<HEADER><TITLE>{{ .Current.AlbumTitle }}</TITLE></HEADER>
		<BODY {{ .App.BodyArgs }}>
			<H3>{{ .Current.AlbumTitle }}</H3>
			{{ .PathInfo }}
			{{ range .Dirs }}
			<dl>
			  {{ $.HandleDirs . "" 0}}
			</dl>
			{{ end }}
		</BODY>
	</HTML>
	`))
}

func (a Album) handleThumbnail(w http.ResponseWriter, req *http.Request, app *App, albumName, pathInfo string) {
	config := app.Default
	albumConfig, ok := app.Albums[albumName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	Merge(&config, &albumConfig)
	thumbDir := config.ThumbDir
	fullFilename := fmt.Sprintf("%s/%s", thumbDir, pathInfo)
	_, err := os.Stat(fullFilename)

	// err just means we need to create it
	if err != nil {
		currentPath := ""
		// First make sure the directories are all there
		paths := strings.Split(fullFilename, "/")
		for idx, path := range paths {
			if idx == 0 || idx == len(paths)-1 {
				continue
			}

			currentPath += "/" + path
			_, err = os.Stat(currentPath)
			if err != nil {
				err := os.Mkdir(currentPath, 0775)
				if err != nil {
					fmt.Printf("Error creating directory %s:%v\n", currentPath, err)
				}
			}
		}

		if IsImageFile(pathInfo) {
			img, err := imaging.Open(fmt.Sprintf("%s/%s", config.AlbumDir, cleanTn(pathInfo)), imaging.AutoOrientation(true))
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
			imaging.Save(dstImage, fullFilename)
		} else {
			// Must be video, need to figure out the original filename and save a frame
			clean := cleanTn(pathInfo)
			prefix := strings.TrimSuffix(clean, filepath.Ext(clean))
			sourceGlob := fmt.Sprintf("%s/%s*", config.AlbumDir, prefix)
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
	}

	http.ServeFile(w, req, fullFilename)
}

func (a Album) Footer() string {
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

func (a App) SortedAlbumTitles() []AlbumTitle {
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
	return index > 0 && index%t.NumberOfColumns == 0
}

func (t TemplateSource) IsImageFile(filename string) bool {
	return IsImageFile(filename)
}

func (t TemplateSource) AsPngFilename(filename string) string {
	return ChangeExtension(filename, "png")
}

func (t TemplateSource) HandleDirs(f os.DirEntry, subdir string, depth int) string {
	fmt.Printf("Called HandleDirs with root:%s, pathInfo:%s, f:%s, subdir:%s, depth:%d\n",
		t.Current.AlbumDir, t.PathInfo, f.Name(), subdir, depth)

	// Check directory to see if there are sub-directories
	newSubDir := f.Name()
	if subdir != "" {
		newSubDir = subdir + "/" + f.Name()
	}
	dir := t.Current.AlbumDir
	if t.PathInfo == "" {
		dir += "/" + newSubDir
	} else {
		dir += "/" + t.PathInfo + "/" + newSubDir
	}

	children := ""
	fmt.Printf("Checking dir %s\n", dir)
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

	return fmt.Sprintf(`<dt><a href="%s%s/">%s</a></dt>`,
		t.Root, newSubDir, beautify(f.Name())) + children
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
		extraTitle = `<CENTER>{{ .Current.AlbumTitle }} - {{ .FullTitle }}</CENTER>`
		height = "150"
	}
	return `
	<HTML>
		<HEADER><TITLE>{{ .PageTitle }}</TITLE></HEADER>
		<BODY {{ .Current.BodyArgs }}>` + extraTitle +
		`<HR />
		<CENTER>
		  <div style="overflow: auto; height: calc(100vh - ` + height + `px)">
		  {{ .CaptionHtml }}
		  <TABLE BORDER={{ .Current.OutsideTableBorder }}>	`
}

func pictureDirFooter() string {
	return `
	  </div>
	  </TABLE>
	</CENTER>
	<HR>
	<CENTER>Slide Show: <a href="?slide_show=sm">small</a> | <a href="?slide_show=med">medium</a> | <a href="?slide_show=lg">large</a> | <a href="?slide_show=full">full sized</a><br>
			All Images: <a href="?all_full_images=sm">small</a> | <a href="?all_full_images=med">medium</a> | <a href="?all_full_images=lg">large</a> | <a href="?all_full_images=full">full sized</a><br>
			<a href="./">Back to thumbnails</a><br>
			<a href="/{{ .BasePath }}/albums/">Back to {{ .Current.AlbumTitle }}</a>
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

func LoadConfigFile() (*App, error) {
	in, err := os.Open(CONFIG_FILENAME)
	if err != nil {
		return nil, err
	}

	defer in.Close()
	decoder := yaml.NewDecoder(in)
	var app App
	err = decoder.Decode(&app)
	if err != nil {
		return nil, err
	}

	file, err := os.Stat(CONFIG_FILENAME)
	if err == nil {
		app.Timestamp = file.ModTime()
	}

	return &app, nil
}
