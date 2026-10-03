# gosgal
static html gallery creator written in go

Creates an album page per folder and a scrollable timeline of all pictures by
date, with WebP thumbnails and a [PhotoSwipe](https://photoswipe.com) lightbox.
Originals are linked, never copied or modified.

Requires `vipsthumbnail` and `vipsheader` (libvips, with HEIF support for HEIC files).

    gosgal [options] outputdir picturedir

    -base string    URL path the output directory is served at (default "/")
    -title string   gallery title (default: name of the picture directory)
    -j int          number of parallel thumbnail jobs (default: number of CPUs)
    -clean          remove thumbnails and links of pictures that no longer exist
    -thumb          force thumbnail creation
    -verbose        verbose output

Metadata is cached in `outputdir/_gosgal/cache.json`, so later runs only look at
new or changed pictures. Dates come from EXIF, then from dates in the file or
folder name, then from the file modification time.
