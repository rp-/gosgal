package main

import (
	"strings"
	"testing"
	"time"
)

// album builds a chain of nested albums from a relative path.
func album(rel string) *Album {
	a := &Album{Name: "root"}
	path := ""
	for _, part := range strings.Split(rel, "/") {
		if path != "" {
			path += "/"
		}
		path += part
		a = &Album{Rel: path, Name: part, Parent: a}
	}
	return a
}

func TestResolveDate(t *testing.T) {
	mtime := time.Date(2015, 3, 1, 12, 0, 0, 0, time.Local).Unix()
	tests := []struct {
		rel, name, exif string
		date, src       string
	}{
		// camera clock never set
		{"2019/Marrakesch (29.4.-04.05.2019)/Fotos Irene", "IMG_1599.JPG", "2000-01-01T00:00:00", "2019-05-04", "folder"},
		{"2019/Marrakesch (29.4.-04.05.2019)", "IMG_1.JPG", "2019-04-30T10:00:00", "2019-04-30", "exif"},
		{"2009", "DSC1.JPG", "2008-12-30T20:00:00", "2008-12-30", "exif"},
		{"2021/Weißensee", "YIAC0129.JPG", "2014-01-04T10:00:00", "2021-01-01", "year"},
		// numbers in file names are not years when the folder has one
		{"2009/Yearbook/Irene", "1950.jpg", "", "2009-01-01", "year"},
		{"2022/6", "IMG_20220831_152752.jpg", "", "2022-08-31", "name"},
		{"2012/Kieser/bearbeitet #3/Jpeg 1920 pix Adobe RGB 1998", "a.jpg", "", "2012-01-01", "year"},
		{"2015/Urlaub", "x.jpg", "", "2015-03-01", "mtime"},
		// no folder date: EXIF is trusted as is
		{"Jester", "a.jpg", "2001-01-17T09:00:00", "2001-01-17", "exif"},
	}
	for _, tc := range tests {
		p := &Photo{Name: tc.name, Album: album(tc.rel)}
		p.Exif = tc.exif
		p.MTime = mtime
		resolveDate(p)
		if p.Date[:10] != tc.date || p.DateSrc != tc.src {
			t.Errorf("%s/%s: got %s (%s), want %s (%s)", tc.rel, tc.name, p.Date, p.DateSrc, tc.date, tc.src)
		}
	}
}

func TestAlbumOrder(t *testing.T) {
	root := &Album{Name: "root"}
	add := func(name string, dates ...string) {
		a := &Album{Rel: name, Name: name, Parent: root}
		for _, d := range dates {
			a.Photos = append(a.Photos, &Photo{Name: d, Album: a, ok: true, Meta: Meta{Date: d}})
		}
		root.Children = append(root.Children, a)
	}
	add("S4", "2013-07-24T00:00:00", "2014-06-01T00:00:00", "2019-12-26T00:00:00")
	add("2014alt", "2011-10-13T00:00:00", "2014-04-23T00:00:00")
	add("2019", "2000-01-01T00:00:00", "2019-05-01T00:00:00", "2019-06-01T00:00:00")
	add("2014", "2014-01-01T00:00:00")
	add("2009", "1950-01-01T00:00:00", "2009-06-01T00:00:00")
	add("Jpeg 1920 pix", "2012-01-01T00:00:00", "2012-02-01T00:00:00")
	root.finalize()

	var got []string
	for _, c := range root.Children {
		got = append(got, c.Name)
	}
	want := "2009 Jpeg 1920 pix 2014 2014alt S4 2019"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}
