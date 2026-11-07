package album

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHtml(t *testing.T) {
	var tests = []struct {
		input    string
		wantHtml string
		wantMap  map[string]string
	}{
		{
			"HtmlOnlyTest",
			"HtmlOnlyTest\n",
			make(map[string]string),
		},
		{
			"HtmlWithEND\n__END__\n",
			"HtmlWithEND\n",
			make(map[string]string),
		},
		{
			"HtmlWithOneCaption\n__END__\na:b",
			"HtmlWithOneCaption\n",
			map[string]string{"a": "b"},
		},
		{
			`
<H1>My Birthday Party</H1>

<center>This is me at my Birthday Party!.</center>

__END__
pieinface.gif: Here's me getting hit the face with a pie.
john5.jpg: This is <A HREF="mailto:johndoe@nowhere.com">John</A>
`,
			`
<H1>My Birthday Party</H1>

<center>This is me at my Birthday Party!.</center>

`,
			map[string]string{
				"pieinface.gif": " Here's me getting hit the face with a pie.",
				"john5.jpg":     ` This is <A HREF="mailto:johndoe@nowhere.com">John</A>`,
			},
		},
	}
	for _, test := range tests {
		reader := strings.NewReader(test.input)
		readCaption := NewCaptionFile(reader)
		if readCaption == nil {
			t.Error("NewCaptionFile should not return nil")
		}
		if readCaption.Html != test.wantHtml {
			t.Errorf("With input %s was expecting %s, but got %s", test.input, test.wantHtml, readCaption.Html)
		}
		if len(readCaption.CaptionMap) != len(test.wantMap) {
			t.Errorf("Expecting map to be of length %d, was %d", len(test.wantMap), len(readCaption.CaptionMap))
		}
		for key, val := range test.wantMap {
			if readCaption.CaptionMap[key] != val {
				t.Errorf("Expecting caption map of key %s to be value %s, was %s", key, val, readCaption.CaptionMap[key])
			}
		}
	}
}

func TestIsImageFileIncludesPNG(t *testing.T) {
	for _, filename := range []string{"Scan_0052.png", "Scan_0053.PNG"} {
		if !IsImageFile(filename) {
			t.Errorf("IsImageFile(%q) = false, want true", filename)
		}
	}
}

func TestAlbumRejectsPathTraversal(t *testing.T) {
	workingDir := t.TempDir()
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDir) })

	albumsDir := filepath.Join(workingDir, "albums")
	albumDir := filepath.Join(albumsDir, "photos")
	thumbDir := filepath.Join(albumsDir, "thumbs")
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumsDir, "secret.txt"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("appconfig.yaml", []byte(fmt.Sprintf("port: 9000\nalbumsDir: %q\n", albumsDir)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumsDir, "albumsconfig.yaml"), []byte("albums:\n  test:\n    albumTitle: Test\n    albumDir: photos\n    thumbDir: thumbs\n"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, requestPath := range []string{
		"/test/albums/../secret.txt",
		"/test/thumbs/../secret.txt",
	} {
		req := httptest.NewRequest("GET", requestPath, nil)
		res := httptest.NewRecorder()
		(Album{}).ServeHTTP(res, req)

		if res.Code == 200 && strings.Contains(res.Body.String(), "private") {
			t.Errorf("request %q exposed a file outside the configured album roots", requestPath)
		}
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	if _, err := safeJoin(root, "../secret.txt"); err == nil {
		t.Fatal("safeJoin accepted a path outside the configured root")
	}

	joined, err := safeJoin(root, "2024/photo.jpg")
	if err != nil {
		t.Fatalf("safeJoin rejected a path inside the configured root: %v", err)
	}
	if want := filepath.Join(root, "2024", "photo.jpg"); joined != want {
		t.Fatalf("safeJoin returned %q, want %q", joined, want)
	}
}

func TestDirectoryTemplateEscapesAlbumTitle(t *testing.T) {
	source := TemplateSource{
		AlbumsConfig: &AlbumsConfig{BodyArgs: `bgcolor="#000000"`},
		AlbumConfig:  AlbumConfig{AlbumTitle: `<script>alert(1)</script>`},
	}

	var output bytes.Buffer
	if err := (&Album{}).generateDirs().Execute(&output, source); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `<script>alert(1)</script>`) {
		t.Fatal("directory template rendered an album title as executable HTML")
	}
}
