package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		base    string
		title   string
		jobs    int
		force   bool
		clean   bool
		verbose bool
	)
	flag.StringVar(&base, "base", "/", "URL path the output directory is served at")
	flag.StringVar(&title, "title", "", "gallery title (default: name of the picture directory)")
	flag.IntVar(&jobs, "j", runtime.NumCPU(), "number of parallel thumbnail jobs")
	flag.BoolVar(&force, "thumb", false, "force thumbnail creation")
	flag.BoolVar(&clean, "clean", false, "remove thumbnails and links of pictures that no longer exist")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "gosgal [options] outputdir picturedir")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	out, err := filepath.Abs(flag.Arg(0))
	check(err)
	root, err := filepath.Abs(flag.Arg(1))
	check(err)
	if title == "" {
		title = filepath.Base(root)
	}

	started := time.Now()
	album := scanTree(root, "", nil, map[string]bool{})
	if album == nil {
		fmt.Fprintf(os.Stderr, "no pictures found in %s\n", root)
		os.Exit(1)
	}
	album.Name = title
	photos := album.allPhotos()
	if verbose {
		fmt.Printf("found %d pictures\n", len(photos))
	}

	cachePath := filepath.Join(out, assetDir, "cache.json")
	check(os.MkdirAll(filepath.Dir(cachePath), 0755))
	cache := loadCache(cachePath)

	var (
		done, generated, failed atomic.Int64
		wg                      sync.WaitGroup
	)
	queue := make(chan *Photo)
	for range max(jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range queue {
				key := filepath.Join(p.Album.Rel, p.Name)
				outDir := filepath.Join(out, p.Album.Rel)
				cached, haveCache := cache[key]
				var n int
				err := os.MkdirAll(outDir, 0755)
				if err == nil {
					n, err = process(p, outDir, cached, haveCache, force)
				}
				generated.Add(int64(n))
				if err != nil {
					failed.Add(1)
					fmt.Fprintf(os.Stderr, "warning: %s: %v\n", key, err)
				} else {
					p.ok = true
				}
				if d := done.Add(1); verbose && d%1000 == 0 {
					fmt.Printf("%d/%d pictures, %d thumbnails generated\n", d, len(photos), generated.Load())
				}
			}
		}()
	}
	for _, p := range photos {
		queue <- p
	}
	close(queue)
	wg.Wait()

	newCache := make(map[string]Meta, len(photos))
	for _, p := range photos {
		if p.ok {
			newCache[filepath.Join(p.Album.Rel, p.Name)] = p.Meta
		}
	}
	check(saveCache(cachePath, newCache))

	if clean {
		n, size := cleanOutput(album, out, verbose)
		fmt.Printf("removed %d stale files, %.1f MiB\n", n, float64(size)/(1<<20))
	}

	if !album.finalize() {
		fmt.Fprintln(os.Stderr, "no pictures could be processed")
		os.Exit(1)
	}
	g := &Gallery{Out: out, Base: base, Title: title}
	check(g.writeAssets())
	check(g.writeAlbum(album))
	check(g.writeTimeline(album))

	if verbose || failed.Load() > 0 {
		fmt.Printf("%d pictures, %d thumbnails generated, %d failed, took %s\n",
			len(photos), generated.Load(), failed.Load(), time.Since(started).Round(time.Second))
	}
}

// cleanOutput removes files gosgal created for pictures that are gone: tn_*
// files and symlinks without a matching picture. Anything else is left alone.
func cleanOutput(a *Album, out string, verbose bool) (int, int64) {
	var n int
	var size int64
	for _, c := range a.Children {
		cn, cs := cleanOutput(c, out, verbose)
		n += cn
		size += cs
	}

	keep := map[string]bool{}
	for _, p := range a.Photos {
		keep[p.Name] = true
		keep[gridName(p.Name)] = true
		keep[mediumName(p.Name)] = true
	}
	dir := filepath.Join(out, a.Rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return n, size
	}
	for _, e := range entries {
		name := e.Name()
		if keep[name] {
			continue
		}
		symlink := e.Type()&os.ModeSymlink != 0
		if !symlink && !(e.Type().IsRegular() && strings.HasPrefix(name, "tn_")) {
			continue
		}
		path := filepath.Join(dir, name)
		if symlink {
			// only links to pictures, not e.g. the photos -> /var/www/html/photos helper
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				continue
			}
		} else if fi, err := e.Info(); err == nil {
			size += fi.Size()
		}
		if verbose {
			fmt.Printf("removing %s\n", path)
		}
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		n++
	}
	return n, size
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
