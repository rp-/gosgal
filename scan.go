package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Album is a folder of the picture tree that contains pictures somewhere below it.
type Album struct {
	Rel      string // path relative to the picture root, "" for the root
	Name     string
	Dir      string // absolute source directory
	Parent   *Album
	Children []*Album
	Photos   []*Photo

	// aggregated over the whole subtree, filled by finalize()
	Count   int
	Start   string
	End     string
	Cover   *Photo
	SortKey string
}

// Photo is a single picture file.
type Photo struct {
	Src   string // absolute source path
	Name  string // file name
	Album *Album
	Meta
	ok bool // metadata and thumbnails are available
}

func isSupported(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".heic", ".heif", ".png":
		return true
	}
	return false
}

func isHeif(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".heic", ".heif":
		return true
	}
	return false
}

func skipEntry(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@")
}

// scanTree reads the picture tree below dir. Symlinked directories are followed,
// cycles are cut using the resolved real path.
func scanTree(dir, rel string, parent *Album, seen map[string]bool) *Album {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", dir, err)
		return nil
	}
	if seen[real] {
		return nil
	}
	seen[real] = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return nil
	}

	// originals are linked by their real path, so the web server doesn't need
	// access to whatever symlinks led here
	a := &Album{Rel: rel, Name: filepath.Base(dir), Dir: real, Parent: parent}
	for _, e := range entries {
		name := e.Name()
		if skipEntry(name) {
			continue
		}
		path := filepath.Join(real, name)
		fi, err := os.Stat(path) // follows symlinks
		if err != nil {
			continue
		}
		if fi.IsDir() {
			if c := scanTree(path, filepath.Join(rel, name), a, seen); c != nil {
				a.Children = append(a.Children, c)
			}
		} else if fi.Mode().IsRegular() && isSupported(name) {
			a.Photos = append(a.Photos, &Photo{Src: path, Name: name, Album: a})
		}
	}

	if len(a.Photos) == 0 && len(a.Children) == 0 {
		return nil
	}
	return a
}

// allPhotos returns all photos of the subtree.
func (a *Album) allPhotos() []*Photo {
	ps := append([]*Photo{}, a.Photos...)
	for _, c := range a.Children {
		ps = append(ps, c.allPhotos()...)
	}
	return ps
}

// finalize drops photos that failed to process, sorts photos and albums by date
// and computes the aggregated values. Returns false if the album ended up empty.
func (a *Album) finalize() bool {
	photos := a.Photos[:0]
	for _, p := range a.Photos {
		if p.ok {
			photos = append(photos, p)
		}
	}
	a.Photos = photos
	sort.SliceStable(a.Photos, func(i, j int) bool {
		if a.Photos[i].Date != a.Photos[j].Date {
			return a.Photos[i].Date < a.Photos[j].Date
		}
		return a.Photos[i].Name < a.Photos[j].Name
	})

	children := a.Children[:0]
	for _, c := range a.Children {
		if c.finalize() {
			children = append(children, c)
		}
	}
	a.Children = children
	sort.SliceStable(a.Children, func(i, j int) bool {
		if a.Children[i].SortKey != a.Children[j].SortKey {
			return a.Children[i].SortKey < a.Children[j].SortKey
		}
		return a.Children[i].Name < a.Children[j].Name
	})

	a.Count = len(a.Photos)
	if len(a.Photos) > 0 {
		a.Cover = a.Photos[0]
		a.Start = a.Photos[0].Date
		a.End = a.Photos[len(a.Photos)-1].Date
	}
	for _, c := range a.Children {
		a.Count += c.Count
		if a.Cover == nil {
			a.Cover = c.Cover
		}
		if a.Start == "" || c.Start < a.Start {
			a.Start = c.Start
		}
		if c.End > a.End {
			a.End = c.End
		}
	}
	if a.Count > 0 {
		a.SortKey = a.sortKey()
	}
	return a.Count > 0
}

// sortKey orders albums by the date in the folder name if it matches the
// photos, or else by the median photo date, so a few misdated photos don't
// move the whole album.
func (a *Album) sortKey() string {
	photos := a.allPhotos()
	dates := make([]string, len(photos))
	for i, p := range photos {
		dates[i] = p.Date
	}
	sort.Strings(dates)
	median := dates[len(dates)/2]

	m := parseDate(median)
	if t, ok := dateFromFolderName(a.Name); ok && plausible(m, t, true) {
		return t.Format(dateLayout)
	}
	if y := yearFromName(a.Name); y != 0 {
		if t := time.Date(y, 1, 1, 0, 0, 0, 0, time.Local); plausible(m, t, false) {
			return t.Format(dateLayout)
		}
	}
	return median
}
