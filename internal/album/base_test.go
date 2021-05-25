package album

import (
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
