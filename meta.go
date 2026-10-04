package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"
)

const dateLayout = "2006-01-02T15:04:05"

// bump if the meaning of cached values changes
const cacheVersion = 3

// Meta is everything we need to know about a photo to render it. It is cached
// between runs, keyed by the path relative to the picture root.
type Meta struct {
	Size  int64 `json:"size"`
	MTime int64 `json:"mtime"`
	// display size of the original (orientation applied)
	W int `json:"w"`
	H int `json:"h"`
	// size of the grid thumbnail
	TW int `json:"tw"`
	TH int `json:"th"`
	// size of the medium version
	MW int `json:"mw"`
	MH int `json:"mh"`
	// capture date from EXIF if there is a sane one, "2006-01-02T15:04:05"
	Exif string `json:"exif,omitempty"`
	// the date used for sorting, see resolveDate; same format as Exif
	Date string `json:"date"`
	// where the date came from: exif, name, folder, mtime, year
	DateSrc string `json:"dsrc"`
}

type cacheFile struct {
	Version int             `json:"version"`
	Photos  map[string]Meta `json:"photos"`
}

func loadCache(path string) map[string]Meta {
	var c cacheFile
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	if err != nil || c.Version != cacheVersion || c.Photos == nil {
		return map[string]Meta{}
	}
	return c.Photos
}

func saveCache(path string, photos map[string]Meta) error {
	b, err := json.Marshal(cacheFile{Version: cacheVersion, Photos: photos})
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// thumbnail file names, next to the symlink of the original in the output folder

func gridName(name string) string   { return "tn_h_" + name + ".webp" }
func mediumName(name string) string { return "tn_m_" + name + ".webp" }

func vipsthumbnail(src, dst, size, opts string) error {
	ext := filepath.Ext(dst)
	tmp := strings.TrimSuffix(dst, ext) + ".tmp" + ext
	out, err := exec.Command("vipsthumbnail", "-s", size, "-o", tmp+opts, src).CombinedOutput()
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("vipsthumbnail %s: %v: %s", src, err, strings.TrimSpace(string(out)))
	}
	return os.Rename(tmp, dst)
}

// needsUpdate reports whether a derived file is missing or older than its source.
func needsUpdate(dst string, srcMTime time.Time) bool {
	fi, err := os.Stat(dst)
	return err != nil || fi.ModTime().Before(srcMTime)
}

func imageSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return c.Width, c.Height, nil
}

func readExif(path string) *exif.Exif {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return nil
	}
	return x
}

func exifOrientation(x *exif.Exif) int {
	if x == nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return v
}

func saneDate(t time.Time) bool {
	return t.Year() >= 1980 && t.Before(time.Now().Add(24*time.Hour))
}

func exifDate(x *exif.Exif) (time.Time, bool) {
	if x == nil {
		return time.Time{}, false
	}
	t, err := x.DateTime()
	if err != nil || !saneDate(t) {
		return time.Time{}, false
	}
	return t, true
}

// heifExifDate asks libvips for the capture date, goexif can't read HEIF.
func heifExifDate(path string) (time.Time, bool) {
	out, err := exec.Command("vipsheader", "-f", "exif-ifd2-DateTimeOriginal", path).Output()
	if err != nil || len(out) < 19 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006:01:02 15:04:05", string(out[:19]), time.Local)
	if err != nil || !saneDate(t) {
		return time.Time{}, false
	}
	return t, true
}

var (
	// 2014-06-08 15.11.54, IMG_20140608_151154, IMG-20140608-WA0001
	reNameDate = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})[-_.]?(0[1-9]|1[0-2])[-_.]?(0[1-9]|[12]\d|3[01])(?:[-_. T]?([01]\d|2[0-3])[-_.:]?([0-5]\d)[-_.:]?([0-5]\d))?`)
	// Korfu (13.09.2005-22.09.2005), 26.07.2008
	reFolderDate = regexp.MustCompile(`(?:^|\D)(\d{1,2})\.(\d{1,2})\.((?:19|20)\d{2})(?:\D|$)`)
	reYear       = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})(?:\D|$)`)
)

func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// mkDate builds a date and rejects invalid ones like 31.02.
func mkDate(y, m, d, hh, mm, ss int) (time.Time, bool) {
	t := time.Date(y, time.Month(m), d, hh, mm, ss, 0, time.Local)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d || !saneDate(t) {
		return time.Time{}, false
	}
	return t, true
}

func dateFromFileName(name string) (time.Time, bool) {
	m := reNameDate.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	return mkDate(atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6]))
}

func dateFromFolderName(name string) (time.Time, bool) {
	for _, m := range reFolderDate.FindAllStringSubmatch(name, -1) {
		if t, ok := mkDate(atoi(m[3]), atoi(m[2]), atoi(m[1]), 0, 0, 0); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func yearFromName(name string) int {
	if m := reYear.FindStringSubmatch(name); m != nil {
		return atoi(m[1])
	}
	return 0
}

// folderDate returns the date of the nearest folder that has a full date in its
// name, or else January 1st of the outermost folder with a year in its name.
// Outermost, because deeper folders have names like "Jpeg 1920 pix Adobe RGB 1998".
func folderDate(a *Album) (t time.Time, full, ok bool) {
	year := 0
	for ; a != nil && a.Rel != ""; a = a.Parent {
		if t, ok := dateFromFolderName(a.Name); ok {
			return t, true, true
		}
		if y := yearFromName(a.Name); y != 0 {
			year = y
		}
	}
	if year != 0 {
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.Local), false, true
	}
	return time.Time{}, false, false
}

// plausible reports whether t fits the folder date: within a year of a full
// date, or at most one year off a year folder.
func plausible(t, folder time.Time, full bool) bool {
	if full {
		d := t.Sub(folder)
		return d > -366*24*time.Hour && d < 366*24*time.Hour
	}
	d := t.Year() - folder.Year()
	return d >= -1 && d <= 1
}

// resolveDate picks the date of a photo. EXIF wins unless it is far off the
// folder date, which happens with cameras whose clock was never set.
func resolveDate(p *Photo) {
	mtime := time.Unix(p.MTime, 0)
	folder, full, haveFolder := folderDate(p.Album)
	set := func(t time.Time, src string) {
		p.Date = t.Format(dateLayout)
		p.DateSrc = src
	}

	if p.Exif != "" {
		if t := parseDate(p.Exif); !haveFolder || plausible(t, folder, full) {
			set(t, "exif")
			return
		}
	}
	if t, ok := dateFromFileName(p.Name); ok {
		set(t, "name")
		return
	}
	switch {
	case haveFolder && full:
		set(folder, "folder")
	case haveFolder && mtime.Year() == folder.Year():
		set(mtime, "mtime")
	case haveFolder:
		set(folder, "year")
	default:
		if y := yearFromName(p.Name); y != 0 && y != mtime.Year() {
			set(time.Date(y, 1, 1, 0, 0, 0, 0, time.Local), "year")
		} else {
			set(mtime, "mtime")
		}
	}
}

// process makes sure thumbnails and the symlink to the original exist in outDir
// and fills p.Meta, reusing the cached entry if the source didn't change.
// Returns the number of generated thumbnails.
func process(p *Photo, outDir string, cached Meta, haveCache, force bool) (int, error) {
	fi, err := os.Stat(p.Src)
	if err != nil {
		return 0, err
	}
	mtime := fi.ModTime()

	link := filepath.Join(outDir, p.Name)
	if cur, err := os.Readlink(link); err != nil || cur != p.Src {
		os.Remove(link)
		if err := os.Symlink(p.Src, link); err != nil {
			return 0, err
		}
	}

	gridPath := filepath.Join(outDir, gridName(p.Name))
	medPath := filepath.Join(outDir, mediumName(p.Name))
	generated := 0
	if force || needsUpdate(gridPath, mtime) {
		if err := vipsthumbnail(p.Src, gridPath, "1200x400", "[Q=80,strip]"); err != nil {
			return generated, err
		}
		generated++
	}
	if force || needsUpdate(medPath, mtime) {
		if err := vipsthumbnail(p.Src, medPath, "1600", "[Q=80,strip]"); err != nil {
			return generated, err
		}
		generated++
	}

	if haveCache && generated == 0 && cached.Size == fi.Size() && cached.MTime == mtime.Unix() {
		p.Meta = cached
		resolveDate(p)
		return 0, nil
	}

	m := Meta{Size: fi.Size(), MTime: mtime.Unix()}
	if m.TW, m.TH, err = imageSize(gridPath); err != nil {
		return generated, fmt.Errorf("%s: %v", gridPath, err)
	}
	if m.MW, m.MH, err = imageSize(medPath); err != nil {
		return generated, fmt.Errorf("%s: %v", medPath, err)
	}

	var date time.Time
	var ok bool
	if isHeif(p.Name) {
		// browsers can't show HEIF, the medium version stands in for the original
		m.W, m.H = m.MW, m.MH
		date, ok = heifExifDate(p.Src)
	} else {
		x := readExif(p.Src)
		if m.W, m.H, err = imageSize(p.Src); err != nil {
			m.W, m.H = m.MW, m.MH
		} else if o := exifOrientation(x); o >= 5 && o <= 8 {
			m.W, m.H = m.H, m.W
		}
		date, ok = exifDate(x)
	}
	if ok {
		m.Exif = date.Format(dateLayout)
	}

	p.Meta = m
	resolveDate(p)
	return generated, nil
}
