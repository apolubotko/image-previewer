package proxy

import (
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/apolubotko/image-previewer/internal/storage"
	"github.com/nfnt/resize"

	log "github.com/sirupsen/logrus"
)

const imagePath = "/tmp"

type Server struct {
	Config *Config
	cache  *storage.Cache
}

type ServeHandler struct {
}

type ImageObj struct {
	width  string
	height string
	url    string
}

func NewInstance(config *Config) (*Server, error) {
	return &Server{
		Config: config,
	}, nil
}

func (s *Server) Start() {
	log.Info("Starting ...")
	handler := &ServeHandler{}

	server := &http.Server{
		Addr:         ":" + s.Config.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func (h *ServeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var path, height, width, reqUrl string
	var image *ImageObj
	tokens := strings.Split(r.URL.Path, "/")
	if len(tokens) > 4 {
		path = tokens[1]
		width = tokens[2]
		height = tokens[3]
		reqUrl = strings.Join(tokens[4:], "/")
		u, err := url.Parse(reqUrl)
		checkErr(err)
		if u.Scheme != "http" && u.Scheme != "https" {
			reqUrl = "http://" + reqUrl
		}
		image = &ImageObj{width: width, height: height, url: reqUrl}
		if path == "fill" {
			log.Infof("%s - %s - %s - %s\n", path, height, width, reqUrl)
			processFillRequest(w, r, image)
		}
	} else {
		log.Info("Can't process request - ", r.URL.Path)
	}
}

// http://localhost:8088/img/gopher.jpg
func processFillRequest(w http.ResponseWriter, r *http.Request, img *ImageObj) {
	var name, ext string

	// 1.
	base := path.Base(img.url)
	fileName := strings.Split(base, ".")
	if len(fileName) > 1 {
		name = fileName[0]
		ext = fileName[1]
	}
	resp, err := http.Get(img.url)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	checkErr(err)
	defer resp.Body.Close()

	// 2.
	file, err := os.Create(imagePath + string(os.PathSeparator) + base)
	checkErr(err)
	defer file.Close()

	// 3.
	_, err = io.Copy(file, resp.Body)
	checkErr(err)

	// 4.
	f, err := os.Open(imagePath + string(os.PathSeparator) + base)
	checkErr(err)
	defer f.Close()

	// 4.
	iii, err := jpeg.Decode(f)
	checkErr(err)

	// // 5.
	width, err := strconv.Atoi(img.width)
	checkErr(err)
	height, err := strconv.Atoi(img.height)
	checkErr(err)
	m := resize.Resize(uint(width), uint(height), iii, resize.Lanczos3)

	// 6.
	newFile := imagePath + string(os.PathSeparator) + name + "_" + img.width + "x" + img.height + "." + ext
	out, err := os.Create(newFile)
	checkErr(err)
	defer out.Close()

	// 8.
	err = jpeg.Encode(out, m, nil)
	checkErr(err)

	_, err = out.Seek(0, 0)
	checkErr(err)

	_, err = io.Copy(w, out)
	checkErr(err)

	log.Info("Done ...")
}

func checkErr(err error) {
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
