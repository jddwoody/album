package album

/*
   Copyright 1998-2021 James D Woodgate.  All rights reserved.
   It may be used and modified freely, but I do request that this copyright
   notice remain attached to the file.  You may modify this module as you
   wish, but if you redistribute a modified version, please attach a note
   listing the modifications you have made.
*/

// Missing sm/med/lg links above caption
// When looking at a single image, also show thumbnails on top, with links to same size
// redirect to first image if directory
// 5; URL=/jdw/albums/2001/12(December)/Christmas/DSCN0138.JPG?slide_show=sm

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/disintegration/imaging"
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
	fmt.Printf("url.Path:%s\n", url.Path)
	var tmpl *template.Template
	tmplSource := TemplateSource{App: a.App}

	paths := strings.SplitN(path[1:], "/", 3)
	if len(paths) < 3 {
		// It should always be at least 2, so generate an error
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if paths[1] == "thumbs" {
		// must be a thumbnail, files only
		a.handleThumbnail(w, req, paths[0], paths[2])
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

	if path == "/" {
		tmpl = a.generateTopPage()
		return
	}

	// Paths[0] should match an album id, Paths[1] should be either albums or thumbs
	tmplSource.Root = path
	tmplSource.BasePath = paths[0]
	tmplSource.PathInfo = paths[2]
	if strings.HasSuffix(tmplSource.PathInfo, "/") {
		tmplSource.PathInfo = tmplSource.PathInfo[:len(tmplSource.PathInfo)-1]
	}
	fmt.Printf("0:%s, 1:%s, leftovers:'%s'\n", paths[0], paths[1], tmplSource.PathInfo)
	var ok bool
	tmplSource.Current, ok = a.App.Albums[paths[0]]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
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

	slideShow := req.URL.Query().Get("slide_show")
	if slideShow != "" {
		tmplSource.ActualPath = fmt.Sprintf("/%s/thumbs/%s/%s", tmplSource.BasePath, filepath.Dir(tmplSource.PathInfo), changeSize(slideShow, filepath.Base(tmplSource.PathInfo)))
		tmplSource.BaseFilename = filepath.Base("/" + tmplSource.PathInfo)
	}
	if tmplSource.ActualPath == "" && stat.Mode().IsRegular() {
		http.ServeFile(w, req, albumPathInfo)
		return
	}

	albumDir := albumPathInfo
	if tmplSource.ActualPath != "" {
		albumDir = filepath.Dir(albumPathInfo)
	}
	fileInfos, err := ioutil.ReadDir(albumDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var captionFile *CaptionFile
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			tmplSource.Dirs = append(tmplSource.Dirs, fileInfo)
		} else {
			if fileInfo.Name() == "caption.txt" {
				in, err := os.Open(fmt.Sprintf("%s/%s", albumDir, fileInfo.Name()))
				if err == nil {
					defer in.Close()
					captionFile = NewCaptionFile(in)
				}
			} else {
				tmplSource.Files = append(tmplSource.Files, fileInfo)
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
		tmpl = template.Must(template.New("base").Parse(pictureDirHeader() +
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

	tmplText := ""
	if tmplSource.ActualPath == "" {
		if slideShow != "" && len(tmplSource.Files) > 0 {
			// If there isn't a filename and slideShow is enabled, just call the first picture
			http.Redirect(w, req, fmt.Sprintf("%s/%s?slide_show=%s", tmplSource.Root, tmplSource.Files[0].Name(), slideShow), http.StatusTemporaryRedirect)
			return
		}
		if tmplSource.Current.NumberOfColumns > 0 {
			tmplSource.NumberOfColumns = uint32(tmplSource.Current.NumberOfColumns)
		} else {
			tmplSource.NumberOfColumns = tmplSource.Current.GetDefaultBrowserWidth() / tmplSource.Current.GetThumbnailWidth()
		}
		tmplText = `           <TR>
		{{ range $index,$ele := .Files }}
			{{ if $.NeedNewRow $index}}
		</TR>
		<TR>  
			 {{ end}}
		  <TD ALIGN="center">
			<TABLE BORDER={{ $.Current.InsideTableBorder }}>
			  <TR>
				<TD ALIGN="center"><A HREF="{{ $ele.Name }}"><IMG SRC="/{{ $.BasePath }}/thumbs/{{ $.PathInfo }}/tn__{{ $ele.Name }}" ALT="{{ $ele.Name }}"></A></TD>
			  </TR>
			  <TR>
				<TD ALIGN="center"><A HREF="640x480_{{ $ele.Name }}">Sm</A> <A HREF="800x600_{{ $ele.Name }}">Med</A> </A><A HREF="1024x768_{{ $ele.Name }}">Lg</A><BR>
				{{ $.MakePicTitle $ele.Name }}
				</TD>
			  </TR>
			</TABLE>
		  </TD>
		{{ end }}
		</TR>
`
	} else {
		for idx, fileInfo := range tmplSource.Files {
			if fileInfo.Name() == tmplSource.BaseFilename {
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
				nextName := tmplSource.Files[tmplSource.FileIndex+more+move].Name()
				currentBase := filepath.Base(tmplSource.Root)
				tmplSource.NextSeven = fmt.Sprintf(`<TD ALIGN="right"><A HREF="%s">&gt;Next %d&gt;</A></TD>`,
					fmt.Sprintf("%s/%s", filepath.Dir(tmplSource.Root), fixNextName(currentBase, nextName)), more)
			}
		}

		fmt.Printf("tmplSource:%s\n", tmplSource)
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
		thumbNailLinks := ""
		for i := lowerIndex; i <= upperIndex; i++ {
			filename := tmplSource.Files[i].Name()
			tnImgSrc := fmt.Sprintf("/%s/thumbs/%s/tn__%s", tmplSource.BasePath, filepath.Dir(tmplSource.PathInfo), filename)
			extraTd := ""
			if i == tmplSource.FileIndex {
				extraTd = ` bgcolor="blue"`
			}

			thumbNailLinks += fmt.Sprintf(`<TD%s><A HREF="%s"><IMG SRC="%s" height="60"></A></TD>`, extraTd, fixNextName(currentBase, filename), tnImgSrc)
		}
		tmplText = fmt.Sprintf(`
		<center><TABLE BORDER="0" CELLPADDING="4" CELLSPACING="0"><TR>{{ .PrevSeven }}%s{{ .NextSeven }}</TR></TABLE>
		<HR>
		<CENTER><A HREF="{{ .BaseFilename }}" BORDER="0"><IMG SRC="{{ .ActualPath }}" ALT="{{ .PathInfo }}"></A>
<HR>
<H3>{{ $.MakePicTitle .BaseFilename}}</H3></CENTER>
<HR>`, thumbNailLinks)

		// If it's a slideshow show, set up a refresh
		if slideShow != "" && tmplSource.FileIndex < lastIndex {
			w.Header().Set("Refresh", fmt.Sprintf("%d; URL=%s?slide_show=%s", tmplSource.Current.SlideShowDelay, tmplSource.Files[tmplSource.FileIndex+1].Name(), slideShow))
		}

	}

	// We have files so go ahead and build a table of thumbnails
	tmpl = template.Must(template.New("base").Parse(pictureDirHeader() + tmplText + pictureDirFooter()))
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
			<BR><HR>
			<address>https://github.com/jddwoody/album</address>
		</BODY>
	</HTML>
	`))
}

func (a Album) handleThumbnail(w http.ResponseWriter, req *http.Request, albumName, pathInfo string) {
	config, ok := a.App.Albums[albumName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

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

		img, err := imaging.Open(fmt.Sprintf("%s/%s", config.AlbumDir, cleanTn(pathInfo)))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		filename := path.Base(pathInfo)
		width := int(config.ThumbNailWidth)
		if strings.HasPrefix(filename, "640") {
			width = 640
		} else if strings.HasPrefix(filename, "800") {
			width = 800
		} else if strings.HasPrefix(filename, "1024") {
			width = 1024
		}
		dstImage := imaging.Resize(img, width, 0, imaging.Box)
		imaging.Save(dstImage, fullFilename)
	}

	http.ServeFile(w, req, fullFilename)
}

func (a Album) generateTopPage() *template.Template {
	return template.Must(template.New("base").Parse(`
	<HTML>
		<HEADER><TITLE>Available Albums</TITLE></HEADER>
		<BODY {{ .App.BodyArgs }}>
			<H3>Available Albums</H3>
			{{ range $key, $value := .App.Albums }}
			  <a href="/{{ $key }}/albums/">{{ $value.AlbumTitle }}</a><br>
			{{ end }}
			<BR><HR>
			<address>https://github.com/jddwoody/album</address>
		</BODY>
	</HTML>
	`))
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

func (t TemplateSource) NeedNewRow(index int) bool {
	fmt.Printf("index:%d,need:%t\n", index, index > 0 && uint32(index)%t.NumberOfColumns == 0)
	return index > 0 && uint32(index)%t.NumberOfColumns == 0
}

func (t TemplateSource) HandleDirs(f os.FileInfo, subdir string, depth int) string {
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
	fileInfos, err := ioutil.ReadDir(dir)
	if err == nil {
		for _, fileInfo := range fileInfos {
			if fileInfo.IsDir() {
				children += t.HandleDirs(fileInfo, newSubDir, depth+1)
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

func pictureDirHeader() string {
	return `
	<HTML>
		<HEADER><TITLE>{{ .PageTitle }}</TITLE></HEADER>
		<BODY {{ .App.BodyArgs }}>
		<CENTER>
		  {{ .CaptionHtml }}
		  <TABLE BORDER={{ .Current.OutsideTableBorder }}>	`
}

func pictureDirFooter() string {
	return `
	</TABLE>
	</CENTER>
	<HR>
	<CENTER>Slide Show: <a href="?slide_show=sm">small</a> | <a href="?slide_show=med">medium</a> | <a href="?slide_show=lg">large</a> | <a href="?slide_show=full">full sized</a><br>
			All Images: <a href="?all_full_images=sm">small</a> | <a href="?all_full_images=med">medium</a> | <a href="?all_full_images=lg">large</a> | <a href="?all_full_images=full">full sized</a><br>
			<a href="./">Back to thumbnails</a><br>
			<a href="/{{ .BasePath }}/albums/">Back to {{ .Current.AlbumTitle }}</a>
	</CENTER><BR>
	<HR>
	<address>https://github.com/jddwoody/album</address>
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
