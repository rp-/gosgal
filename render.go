package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/*
var assetFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// assetDir is the folder below the output root for static files and generated data
const assetDir = "_gosgal"

type Gallery struct {
	Out   string // output root
	Base  string // URL path of the output root, with trailing slash
	Title string
	ver   string // asset version for cache busting
}

func escapeURLPath(p string) string {
	var r []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			r = append(r, url.PathEscape(s))
		}
	}
	return "/" + strings.Join(r, "/")
}

func (g *Gallery) assetsURL() string   { return g.Base + assetDir + "/" }
func (g *Gallery) timelineURL() string { return g.Base + "timeline.html" }

func (g *Gallery) albumURL(a *Album) string {
	u := escapeURLPath(g.Base + filepath.ToSlash(a.Rel))
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

func (g *Gallery) writeAssets() error {
	dir := filepath.Join(g.Out, assetDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	h := sha256.New()
	err := fs.WalkDir(assetFS, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := assetFS.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(b)
		return writeFileAtomic(filepath.Join(dir, d.Name()), b)
	})
	g.ver = hex.EncodeToString(h.Sum(nil))[:10]
	return err
}

func (g *Gallery) render(path, tmpl string, data any) error {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, tmpl, data); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes())
}

func parseDate(s string) time.Time {
	t, _ := time.ParseInLocation(dateLayout, s, time.Local)
	return t
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func dateRange(start, end string) string {
	s, e := parseDate(start), parseDate(end)
	switch {
	case sameDay(s, e):
		return s.Format("02.01.2006")
	case s.Year() == e.Year():
		return s.Format("02.01.") + " – " + e.Format("02.01.2006")
	default:
		return s.Format("02.01.2006") + " – " + e.Format("02.01.2006")
	}
}

// page data for templates/album.html

type crumb struct {
	Name, URL string
}

type albumCard struct {
	Name, URL, Cover string
	CoverAspect      template.CSS
	Count            int
	Dates            string
}

type photoItem struct {
	Src, Thumb, Orig string
	// html/template would URL-escape the spaces of a srcset in an attribute whose
	// name contains "src", so the whole attribute is rendered here
	SrcsetAttr          template.HTMLAttr
	W, H, TW, TH        int
	Style               template.CSS
	Name, Date, DateSrc string
}

type albumPage struct {
	Title, Assets, Ver, Root, Timeline string
	Crumbs                             []crumb
	Name                               string
	Count                              int
	Dates                              string
	Albums                             []albumCard
	Photos                             []photoItem
}

type photoURLs struct {
	src, srcset, thumb, orig string
}

func (g *Gallery) photoURLs(p *Photo) photoURLs {
	base := g.albumURL(p.Album)
	orig := base + url.PathEscape(p.Name)
	med := base + url.PathEscape(mediumName(p.Name))
	u := photoURLs{src: orig, thumb: base + url.PathEscape(gridName(p.Name)), orig: orig}
	if isHeif(p.Name) {
		u.src = med
	} else if p.MW < p.W {
		u.srcset = fmt.Sprintf("%s %dw, %s %dw", med, p.MW, orig, p.W)
	}
	return u
}

func srcsetAttr(srcset string) template.HTMLAttr {
	if srcset == "" {
		return ""
	}
	return template.HTMLAttr(`data-pswp-srcset="` + html.EscapeString(srcset) + `"`)
}

func aspect(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 1
	}
	return float64(w) / float64(h)
}

func (g *Gallery) writeAlbum(a *Album) error {
	for _, c := range a.Children {
		if err := g.writeAlbum(c); err != nil {
			return err
		}
	}

	page := albumPage{
		Title:    g.Title,
		Assets:   g.assetsURL(),
		Ver:      g.ver,
		Root:     g.Base,
		Timeline: g.timelineURL() + "#" + a.Start[:7],
		Name:     a.Name,
		Count:    a.Count,
		Dates:    dateRange(a.Start, a.End),
	}
	if a.Parent != nil {
		page.Title = a.Name + " – " + g.Title
	} else {
		page.Timeline = g.timelineURL()
	}
	for p := a.Parent; p != nil; p = p.Parent {
		page.Crumbs = append([]crumb{{p.Name, g.albumURL(p)}}, page.Crumbs...)
	}
	for _, c := range a.Children {
		page.Albums = append(page.Albums, albumCard{
			Name:  c.Name,
			URL:   g.albumURL(c),
			Cover: g.photoURLs(c.Cover).thumb,
			Count: c.Count,
			Dates: dateRange(c.Start, c.End),
		})
	}
	for _, p := range a.Photos {
		u := g.photoURLs(p)
		page.Photos = append(page.Photos, photoItem{
			Src: u.src, SrcsetAttr: srcsetAttr(u.srcset), Thumb: u.thumb, Orig: u.orig,
			W: p.W, H: p.H, TW: p.TW, TH: p.TH,
			Style:   template.CSS(fmt.Sprintf("--a:%.4f", aspect(p.TW, p.TH))),
			Name:    p.Name,
			Date:    p.Date,
			DateSrc: p.DateSrc,
		})
	}

	dir := filepath.Join(g.Out, a.Rel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return g.render(filepath.Join(dir, "index.html"), "album.html", page)
}

// timeline data: _gosgal/timeline/index.json lists months and albums,
// _gosgal/timeline/YYYY-MM.json holds the photos of one month, newest first.

type tlMonth struct {
	Month string `json:"m"`
	Count int    `json:"n"`
}

type tlAlbum struct {
	URL  string `json:"u"`
	Name string `json:"n"`
}

type tlIndex struct {
	Title  string    `json:"title"`
	Months []tlMonth `json:"months"`
	Albums []tlAlbum `json:"albums"`
}

type tlPhoto struct {
	Album   int     `json:"al"`
	Src     string  `json:"s"`            // file name relative to the album URL
	Srcset  string  `json:"ss,omitempty"` // complete srcset
	Thumb   string  `json:"t"`
	Orig    string  `json:"o"`
	W       int     `json:"w"`
	H       int     `json:"h"`
	Aspect  float64 `json:"a"`
	Name    string  `json:"n"`
	Date    string  `json:"d"`
	DateSrc string  `json:"ds"`
}

type timelinePage struct {
	Title, Assets, Ver, Root, Data string
}

func (g *Gallery) writeTimeline(root *Album) error {
	photos := root.allPhotos()
	sort.SliceStable(photos, func(i, j int) bool {
		if photos[i].Date != photos[j].Date {
			return photos[i].Date > photos[j].Date
		}
		return photos[i].Name < photos[j].Name
	})

	idx := tlIndex{Title: g.Title}
	albumIdx := map[*Album]int{}
	months := map[string][]tlPhoto{}
	for _, p := range photos {
		ai, ok := albumIdx[p.Album]
		if !ok {
			ai = len(idx.Albums)
			albumIdx[p.Album] = ai
			idx.Albums = append(idx.Albums, tlAlbum{URL: g.albumURL(p.Album), Name: p.Album.Name})
		}
		base := g.albumURL(p.Album)
		u := g.photoURLs(p)
		m := p.Date[:7]
		if len(months[m]) == 0 {
			idx.Months = append(idx.Months, tlMonth{Month: m})
		}
		months[m] = append(months[m], tlPhoto{
			Album:   ai,
			Src:     strings.TrimPrefix(u.src, base),
			Srcset:  u.srcset,
			Thumb:   strings.TrimPrefix(u.thumb, base),
			Orig:    strings.TrimPrefix(u.orig, base),
			W:       p.W,
			H:       p.H,
			Aspect:  float64(int(aspect(p.TW, p.TH)*10000)) / 10000,
			Name:    p.Name,
			Date:    p.Date,
			DateSrc: p.DateSrc,
		})
	}

	dir := filepath.Join(g.Out, assetDir, "timeline")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	keep := map[string]bool{"index.json": true}
	for i := range idx.Months {
		m := &idx.Months[i]
		m.Count = len(months[m.Month])
		b, err := json.Marshal(months[m.Month])
		if err != nil {
			return err
		}
		name := m.Month + ".json"
		keep[name] = true
		if err := writeFileAtomic(filepath.Join(dir, name), b); err != nil {
			return err
		}
	}
	b, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, "index.json"), b); err != nil {
		return err
	}
	// drop months that no longer have photos
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !keep[e.Name()] {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}

	return g.render(filepath.Join(g.Out, "timeline.html"), "timeline.html", timelinePage{
		Title:  "Timeline – " + g.Title,
		Assets: g.assetsURL(),
		Ver:    g.ver,
		Root:   g.Base,
		Data:   g.assetsURL() + "timeline/",
	})
}
