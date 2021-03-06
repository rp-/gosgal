package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/rwcarlsen/goexif/exif"
)

var html_head string = `<!DOCTYPE html>
<html>
<head>
  <title>/Pictures</title>
  <meta name="viewport" charset="utf-8" content="width=device-width, initial-scale=1.0" />
  <link rel="stylesheet" href="/photos/photoswipe-4.1.3/photoswipe.css">
  <link rel="stylesheet" href="/photos/photoswipe-4.1.3/default-skin/default-skin.css">
  <script src="/photos/photoswipe-4.1.3/photoswipe.min.js"></script>
  <script src="/photos/photoswipe-4.1.3/photoswipe-ui-default.min.js"></script>
  <script defer src="https://use.fontawesome.com/releases/v5.0.8/js/solid.js" integrity="sha384-+Ga2s7YBbhOD6nie0DzrZpJes+b2K1xkpKxTFFcx59QmVPaSA8c7pycsNaFwUK6l" crossorigin="anonymous"></script>
  <script defer src="https://use.fontawesome.com/releases/v5.0.8/js/fontawesome.js" integrity="sha384-7ox8Q2yzO/uWircfojVuCQOZl+ZZBg2D2J5nkpLqzH1HY0C1dHlTKIbpRz/LG23c" crossorigin="anonymous"></script>
  <style>
body {
   margin: 0;
   padding: 0;
   background: #EEE;
   font: 10px/13px 'Lucida Sans',sans-serif;
}
.wrap {
   overflow: hidden;
   margin: 10px;
}
.box {
   float: left;
   position: relative;
   width: 10%;
   padding-bottom: 10%;
}
.boxInner {
   position: absolute;
   left: 10px;
   right: 10px;
   top: 10px;
   bottom: 10px;
   overflow: hidden;
}
.boxInner img {
   width: 100%;
}
.boxInner .titleBox {
   position: absolute;
   bottom: 0;
   left: 0;
   right: 0;
   margin-bottom: -50px;
   background: #000;
   background: rgba(0, 0, 0, 0.5);
   color: #FFF;
   padding: 10px;
   text-align: center;
   -webkit-transition: all 0.3s ease-out;
   -moz-transition: all 0.3s ease-out;
   -o-transition: all 0.3s ease-out;
   transition: all 0.3s ease-out;
}
body.no-touch .boxInner:hover .titleBox, body.touch .boxInner.touchFocus .titleBox {
   margin-bottom: 0;
}
@media only screen and (max-width : 480px) {
   /* Smartphone view: 1 tile */
   .box {
      width: 50%;
      padding-bottom: 50%;
   }
}
@media only screen and (max-width : 650px) and (min-width : 481px) {
   /* Tablet view: 2 tiles */
   .box {
      width: 33.3%;
      padding-bottom: 33.3%;
   }
}
@media only screen and (max-width : 1050px) and (min-width : 651px) {
   /* Small desktop / ipad view: 3 tiles */
   .box {
      width: 20%;
      padding-bottom: 20%;
   }
}
@media only screen and (max-width : 1290px) and (min-width : 1051px) {
   /* Medium desktop: 4 tiles */
   .box {
      width: 10%;
      padding-bottom: 15%;
   }
}

.columns {
  -moz-column-width: 20em;
  -webkit-column-width: 20em;
  column-width: 20em;
  font-size: 1.4em;
  padding: 1em;
}

.columns ul {
  margin: 0;
  padding: 0;
  list-style-type: none;
}

.columns ul li:first-child {
  margin-top: 0px;
}

.columns li {
  padding: 0.2em;
}
  </style>
</head>
<body class="no-touch">
  <!-- Root element of PhotoSwipe. Must have class pswp. -->
  <div class="pswp" tabindex="-1" role="dialog" aria-hidden="true">
      <!-- Background of PhotoSwipe.
           It's a separate element as animating opacity is faster than rgba(). -->
      <div class="pswp__bg"></div>
      <!-- Slides wrapper with overflow:hidden. -->
      <div class="pswp__scroll-wrap">
          <!-- Container that holds slides.
              PhotoSwipe keeps only 3 of them in the DOM to save memory.
              Don't modify these 3 pswp__item elements, data is added later on. -->
          <div class="pswp__container">
              <div class="pswp__item"></div>
              <div class="pswp__item"></div>
              <div class="pswp__item"></div>
          </div>
          <!-- Default (PhotoSwipeUI_Default) interface on top of sliding area. Can be changed. -->
          <div class="pswp__ui pswp__ui--hidden">
              <div class="pswp__top-bar">
                  <!--  Controls are self-explanatory. Order can be changed. -->
                  <div class="pswp__counter"></div>
                  <button class="pswp__button pswp__button--close" title="Close (Esc)"></button>
                  <button class="pswp__button pswp__button--share" title="Share"></button>
                  <button class="pswp__button pswp__button--fs" title="Toggle fullscreen"></button>
                  <button class="pswp__button pswp__button--zoom" title="Zoom in/out"></button>
                  <!-- Preloader demo http://codepen.io/dimsemenov/pen/yyBWoR -->
                  <!-- element will get class pswp__preloader--active when preloader is running -->
                  <div class="pswp__preloader">
                      <div class="pswp__preloader__icn">
                        <div class="pswp__preloader__cut">
                          <div class="pswp__preloader__donut"></div>
                        </div>
                      </div>
                  </div>
              </div>
              <div class="pswp__share-modal pswp__share-modal--hidden pswp__single-tap">
                  <div class="pswp__share-tooltip"></div>
              </div>
              <button class="pswp__button pswp__button--arrow--left" title="Previous (arrow left)">
              </button>
              <button class="pswp__button pswp__button--arrow--right" title="Next (arrow right)">
              </button>
              <div class="pswp__caption">
                  <div class="pswp__caption__center"></div>
              </div>
          </div>
      </div>
  </div>
`

var html_footer string = `	<script>
  var openPhotoSwipe = function(index) {
      var pswpElement = document.querySelectorAll('.pswp')[0];

      // build items array
      var items = %s;

      // define options (if needed)
      var options = {
               // history & focus options are disabled on CodePen
          history: false,
          focus: false,

          showAnimationDuration: 0,
          hideAnimationDuration: 0,
          index: index
      };

      var gallery = new PhotoSwipe( pswpElement, PhotoSwipeUI_Default, items, options);
      gallery.init();
  };

  </script>
</body>
</html>
`

var html_img_div string = `  <div class="box">
	<div class="boxInner">
    	<img src="%s" onclick="openPhotoSwipe(%d)"/>
    	<div class="titleBox">%s</div>
    </div>
  </div>
`

func is_supported(path string) bool {
	lopath := strings.ToLower(path)
	return strings.HasSuffix(lopath, "jpg")
}

func HasPictures(path string) bool {
	allfiles, _ := filepath.Glob(filepath.Join(path, "*"))
	return len(Filter(allfiles, is_supported)) > 0
}

func image_size(path string) (int, int) {
	f, _ := os.Open(path)
	defer f.Close()
	j, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return j.Width, j.Height
}

func image_orientation(path string) int {
	f, _ := os.Open(path)
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return 0
	}

	val, err := x.Get(exif.Orientation)
	if err != nil {
		return 0
	}

	orient_val, _ := val.Int(0)
	if orient_val == 6 {
		return 90
	} else if orient_val == 8 {
		return -90
	}

	return 0
}

func vipsthumbnail(origFile, newFile string, size int, crop bool) (int, int) {
	var args = []string{
		"-s", strconv.Itoa(size),
		"--rotate",
		"-o", newFile,
		origFile,
	}
	if crop {
		args = append(args, "--crop")
	}

	var cmd *exec.Cmd
	path, _ := exec.LookPath("vipsthumbnail")
	cmd = exec.Command(path, args...)
	err := cmd.Run()
	if err != nil {
		fmt.Errorf("Error running vipsthumbnail: %s", args)
	}

	return image_size(newFile)
}

func Filter(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func EscapeUrlPath(urlstr string) string {
	r := make([]string, 0)
	for _, p := range strings.Split(urlstr, "/") {
		if p != "" {
			r = append(r, url.PathEscape(p))
		}
	}
	return "/" + strings.Join(r, "/")
}

func create_index(node FolderNode) {
	if Verbose {
		fmt.Println(fmt.Sprintf("Processing %s...", node.Path))
	}
	folder_path := node.Path
	allfiles, _ := filepath.Glob(folder_path + "/*")
	files := Filter(allfiles, is_supported)

	folder_fragment := folder_path[len(RootPath):]
	output_path := filepath.Join(OutputPath, folder_fragment)
	if !strings.HasSuffix(output_path, "/") {
		output_path += "/"
	}
	link_base_path := output_path
	if BasePath != "" {
		if folder_fragment != "" {
			link_base_path = BasePath + folder_fragment + "/"
		} else {
			link_base_path = BasePath
		}
	}
	os.MkdirAll(output_path, 0755)
	idx_file, _ := os.Create(filepath.Join(output_path, "index.html"))
	defer idx_file.Close()

	idx_file.WriteString(html_head)

	// create folder link list
	idx_file.WriteString("<div class=\"columns\"><ul>")
	parent_picture_node := FindParentPictureNode(&node)
	if parent_picture_node != nil {
		parent_output_path := BasePath + parent_picture_node.Path[len(RootPath):]
		idx_file.WriteString(
			fmt.Sprintf("<li><a href=\"%s\"><i class=\"fas fa-level-up-alt fa-lg\"></i></a></li>",
				EscapeUrlPath(parent_output_path+"/index.html")))
	}

	subfolders := LinkList(node)
	for _, folder := range subfolders {
		idx_file.WriteString(
			fmt.Sprintf("<li><a href=\"%s\">%s</a></li>", EscapeUrlPath(link_base_path+folder[len(RootPath+folder_fragment):]+"/index.html"),
				folder[len(RootPath):]))
	}
	idx_file.WriteString("</ul></div>")

	// create image grid
	type ImageItem struct {
		Src  string `json:"src"`
		Msrc string `json:"msrc"`
		W    int    `json:"w"`
		H    int    `json:"h"`
		Rotate int `json:"rotate,omitempty"`
	}
	var items []ImageItem
	for _, file := range files {
		link_path := EscapeUrlPath(link_base_path + filepath.Base(file))
		symlink_path := filepath.Join(output_path, filepath.Base(file))
		tn_msrc_path := EscapeUrlPath(link_base_path + "tn_msrc_" + filepath.Base(file))
		os.Symlink(file, symlink_path)
		w, h := image_size(file)
		rotation := image_orientation(file)
		if rotation != 0 {
			h, w = w, h
		}
		item := ImageItem{Src: link_path, Msrc: tn_msrc_path, W: w, H: h, Rotate: rotation}
		items = append(items, item)
	}
	b, _ := json.Marshal(items)
	idx_file.WriteString("<div id=\"wrap\">")
	for i, file := range files {
		tn_path := filepath.Join(output_path, "tn_"+filepath.Base(file))
		tn_msrc_path := filepath.Join(output_path, "tn_msrc_"+filepath.Base(file))
		tn_link_path := EscapeUrlPath(link_base_path + filepath.Base(tn_path))
		if _, err := os.Stat(tn_path); os.IsNotExist(err) || forceThumb {
			vipsthumbnail(file, tn_path, 320, true)
			vipsthumbnail(file, tn_msrc_path, 1600, false)
		}
		idx_file.WriteString(fmt.Sprintf(html_img_div, tn_link_path, i, filepath.Base(file)))
	}

	idx_file.WriteString("</div>")
	idx_file.WriteString(fmt.Sprintf(html_footer, b))
}

func CreateIndexes(node FolderNode) {
	for _, c := range node.Children {
		CreateIndexes(*c)
	}

	if node.HasPictures {
		create_index(node)
	}
}

type FolderNode struct {
	Path        string
	Parent      *FolderNode
	Children    []*FolderNode
	HasPictures bool
}

func (fn FolderNode) String() string {
	childstr := make([]string, len(fn.Children))
	for i, c := range fn.Children {
		childstr[i] = c.String()
	}
	return fmt.Sprintf("FolderNode{%s, [%s], %t}", fn.Path, strings.Join(childstr, ","), fn.HasPictures)
}

func BuildFolderTree(path string, parent *FolderNode) *FolderNode {
	allfiles, _ := filepath.Glob(filepath.Join(path, "*"))
	for _, file := range allfiles {
		fi, _ := os.Stat(file)
		if fi.IsDir() {
			cnode := FolderNode{Path: file, Parent: parent, HasPictures: HasPictures(file)}
			parent.Children = append(parent.Children, BuildFolderTree(file, &cnode))
		}
	}
	return parent
}

func LinkList(fn FolderNode) []string {
	l := []string{}
	for _, c := range fn.Children {
		if c.HasPictures {
			l = append(l, c.Path)
		} else {
			l = append(l, LinkList(*c)...)
		}
	}
	return l
}

func FindChildNode(start FolderNode, path string) *FolderNode {
	for _, c := range start.Children {
		if c.Path == path {
			return c
		} else {
			return FindChildNode(*c, path)
		}
	}
	return nil
}

func FindParentPictureNode(start *FolderNode) *FolderNode {
	for start.Parent != nil {
		if start.Parent.HasPictures {
			return start.Parent
		} else {
			start = start.Parent
		}
	}
	return nil
}

var BasePath string
var forceThumb bool
var Verbose bool

func init() {
	flag.BoolVar(&Verbose, "verbose", false, "verbose output")
	flag.BoolVar(&forceThumb, "thumb", false, "force thumbnail creation")
	flag.StringVar(&BasePath, "base", "", "base directory for paths")
}

var OutputPath string
var RootPath string

func main() {
	flag.Parse()

	if BasePath != "" && !strings.HasSuffix(BasePath, "/") {
		BasePath += "/"
	}

	if flag.NArg() != 2 {
		fmt.Println("gosgal [options] outputdir picturedir")
		flag.PrintDefaults()
		os.Exit(1)
	}
	OutputPath = flag.Arg(0)
	RootPath = flag.Arg(1)
	fn := FolderNode{Path: RootPath, Parent: nil, HasPictures: true}
	BuildFolderTree(RootPath, &fn)
	//fmt.Println(fn)
	//ll := LinkList(fn)
	//fmt.Println(ll)
	CreateIndexes(fn)
}
