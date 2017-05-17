package astisub

import (
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// Vars
var (
	regexpTTMLDurationFrames = regexp.MustCompile("\\:[\\d]+$")
)

// TTML represents a TTML
type TTML struct {
	Framerate int            `xml:"frameRate,attr,omitempty"`
	Lang      string         `xml:"lang,attr,omitempty"`
	Regions   []TTMLRegion   `xml:"head>layout>region,omitempty"`
	Styles    []TTMLStyle    `xml:"head>styling>style,omitempty"`
	Subtitles []TTMLSubtitle `xml:"body>div>p"`
	XMLName   xml.Name       `xml:"tt"`
}

// TTMLRegion represents a TTML region
type TTMLRegion struct {
	Extent string `xml:"extent,attr,omitempty"`
	ID     string `xml:"id,attr,omitempty"`
	Origin string `xml:"origin,attr,omitempty"`
	Style  string `xml:"style,attr,omitempty"`
	ZIndex string `xml:"zIndex,attr,omitempty"`
}

// TTMLStyle represents a TTML style
type TTMLStyle struct {
	BackgroundColor string `xml:"backgroundColor,attr,omitempty"`
	Color           string `xml:"color,attr,omitempty"`
	DisplayAlign    string `xml:"displayAlign,attr,omitempty"`
	Extent          string `xml:"extent,attr,omitempty"`
	FontFamily      string `xml:"fontFamily,attr,omitempty"`
	FontSize        string `xml:"fontSize,attr,omitempty"`
	ID              string `xml:"id,attr,omitempty"`
	Origin          string `xml:"origin,attr,omitempty"`
	Style           string `xml:"style,attr,omitempty"`
	TextAlign       string `xml:"textAlign,attr,omitempty"`
}

// TTMLSubtitle represents a TTML subtitle
type TTMLSubtitle struct {
	Begin  *TTMLDuration `xml:"begin,attr"`
	End    *TTMLDuration `xml:"end,attr"`
	ID     string        `xml:"id,attr,omitempty"`
	Region string        `xml:"region,attr,omitempty"`
	Text   []TTMLText    `xml:"span"`
}

// TTMLText represents a TTML text
type TTMLText struct {
	Style    string `xml:"style,attr,omitempty"`
	Sentence string `xml:",chardata"`
}

// TTMLDuration represents a TTML duration
type TTMLDuration struct {
	d                 time.Duration
	frames, framerate int // Framerate is in frame/s
}

// Duration returns the TTML Duration's time.Duration
func (d TTMLDuration) Duration() time.Duration {
	if d.framerate > 0 {
		return d.d + time.Duration(float64(d.frames)/float64(d.framerate)*1e9)*time.Nanosecond
	}
	return d.d
}

// MarshalText implements the TextMarshaler interface
func (t *TTMLDuration) MarshalText() ([]byte, error) {
	return []byte(formatDuration(t.Duration(), ".")), nil
}

// UnmarshalText implements the TextUnmarshaler interface
// Possible formats are:
// - hh:mm:ss.mmm
// - hh:mm:ss:fff (fff being frames)
func (t *TTMLDuration) UnmarshalText(i []byte) (err error) {
	// hh:mm:ss:fff format
	var text = string(i)
	if indexes := regexpTTMLDurationFrames.FindStringIndex(text); indexes != nil {
		// Parse frames
		var s = text[indexes[0]+1 : indexes[1]]
		if t.frames, err = strconv.Atoi(s); err != nil {
			err = errors.Wrapf(err, "atoi %s failed", s)
			return
		}

		// Update text
		text = text[:indexes[0]] + ".000"
	}
	t.d, err = parseDuration(text, ".")
	return
}

// ReadFromTTML parses a .ttml content
// TODO Add region and style to subtitle as well
func ReadFromTTML(i io.Reader) (o *Subtitles, err error) {
	// Init
	o = &Subtitles{}

	// Unmarshal XML
	var ttml TTML
	if err = xml.NewDecoder(i).Decode(&ttml); err != nil {
		return
	}

	// Loop through subtitles
	for _, s := range ttml.Subtitles {
		// Get text
		var text []string
		for _, t := range s.Text {
			text = append(text, t.Sentence)
		}

		// Update framerate
		s.Begin.framerate = ttml.Framerate
		s.End.framerate = ttml.Framerate

		// Append subtitle
		o.Items = append(o.Items, &Subtitle{
			EndAt:   s.End.Duration(),
			StartAt: s.Begin.Duration(),
			Text:    text,
		})
	}
	return
}

// WriteToTTML writes subtitles in .ttml format
func (s Subtitles) WriteToTTML(o io.Writer) (err error) {
	// Do not write anything if no subtitles
	if len(s.Items) == 0 {
		return ErrNoSubtitlesToWrite
	}

	// Loop through items
	var ttml = TTML{}
	for _, sub := range s.Items {
		// Init TTML text
		var text = []TTMLText{}
		for _, t := range sub.Text {
			text = append(text, TTMLText{Sentence: t})
		}

		// Append subtitle
		ttml.Subtitles = append(ttml.Subtitles, TTMLSubtitle{
			Begin: &TTMLDuration{d: sub.StartAt},
			End:   &TTMLDuration{d: sub.EndAt},
			Text:  text,
		})
	}

	// Marshal XML
	var e = xml.NewEncoder(o)
	e.Indent("", "    ")
	return e.Encode(ttml)
}
