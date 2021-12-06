package proxy

import (
	"context"
	"errors"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/apolubotko/image-previewer/internal/storage"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nfnt/resize"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

const (
	imagePath          = "/tmp"
	fillPath           = "/fill"
	sessionName        = "session"
	ctxKeyUser  ctxKey = iota
	ctxKeyRequestID
)

type ctxKey int8

var (
	errEmptyConfig = errors.New("empty config file")
)

type Server struct {
	Config *Config
	cache  storage.Cache
	router *mux.Router
	logger *logrus.Logger
}

type ServeHandler struct {
	cache storage.Cache
}

type ImageObj struct {
	width  string
	height string
	url    string
}

func NewInstance(config *Config) (*Server, error) {
	server := &Server{}
	if config == nil {
		return nil, errEmptyConfig
	}

	server.logger = logrus.New()
	server.router = mux.NewRouter()
	server.Config = config
	server.cache = storage.NewCache(config.CacheSize)

	return server, nil
}

func (s *Server) Start() {
	if err := s.configureLogger(); err != nil {
		s.logger.Fatal(err)
	}

	s.configureRouter()

	log.Info("Starting Proxy Server ...")

	wait := time.Second * 15

	srv := &http.Server{
		Addr:         ":" + s.Config.Port,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		IdleTimeout:  time.Second * 60,
		Handler:      s.router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			s.logger.Error(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	srv.Shutdown(ctx)

	s.logger.Info("shutting down")
	os.Exit(0)
}

func (s *Server) configureRouter() {
	// s.router.Use(s.setRequestID)
	// s.router.Use(s.logRequest)
	// s.router.Use(handlers.CORS(handlers.AllowedOrigins([]string{"*"})))
	s.router.HandleFunc(fillPath, s.handleFilRequest()).
		Methods(http.MethodGet)

	//*********************************************************//
	//														   //
	//		      Add here your handlers and endpoints   	   //
	//														   //
	// Example:												   //
	// 	private.HandleFunc("/user", s.handleUserCreate()).   //
	//	Methods(http.MethodPost)							   //
	//														   //
	//*********************************************************//
}

func (s *Server) handleFilRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("HERE!!!!!!")
		// var path, height, width, reqUrl string
		// var image *ImageObj
		// tokens := strings.Split(r.URL.Path, "/")
		// if len(tokens) > 4 {
		// 	path = tokens[1]
		// 	width = tokens[2]
		// 	height = tokens[3]
		// 	reqUrl = strings.Join(tokens[4:], "/")
		// 	u, err := url.Parse(reqUrl)
		// 	checkErr(err)
		// 	if u.Scheme != "http" && u.Scheme != "https" {
		// 		reqUrl = "http://" + reqUrl
		// 	}
		// 	image = &ImageObj{width: width, height: height, url: reqUrl}
		// 	if path == handlerFilPath {
		// 		h.processFillRequest(w, r, image)
		// 	}
		// } else {
		// 	log.Info("Can't process request - ", r.URL.Path)
		// }

		w.WriteHeader(http.StatusOK)
	}
}

// func (h *ServeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	var path, height, width, reqUrl string
// 	var image *ImageObj
// 	tokens := strings.Split(r.URL.Path, "/")
// 	if len(tokens) > 4 {
// 		path = tokens[1]
// 		width = tokens[2]
// 		height = tokens[3]
// 		reqUrl = strings.Join(tokens[4:], "/")
// 		u, err := url.Parse(reqUrl)
// 		checkErr(err)
// 		if u.Scheme != "http" && u.Scheme != "https" {
// 			reqUrl = "http://" + reqUrl
// 		}
// 		image = &ImageObj{width: width, height: height, url: reqUrl}
// 		if path == handlerFilPath {
// 			h.processFillRequest(w, r, image)
// 		}
// 	} else {
// 		log.Info("Can't process request - ", r.URL.Path)
// 	}
// }

// http://localhost:8088/img/gopher.jpg
func (h *ServeHandler) processFillRequest(w http.ResponseWriter, r *http.Request, img *ImageObj) {
	var name, ext string

	// 1. Step 1
	// gopher.jpg
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

	// Path to requested file name like '/tmp/gopher_500x600.jpg'
	reqFile := imagePath + string(os.PathSeparator) + name + "_" + img.width + "x" + img.height + "." + ext
	// File with base image like '/tmp/gopher.jpg'
	baseFile := imagePath + string(os.PathSeparator) + base

	// 2. Check the cache
	log.Info("Check the cache")
	_, ok := h.cache.Get(storage.Key(reqFile))

	if !ok {
		log.Info("File not found in local cache. Creating ...")

		// 2. Create the base file on local disk
		file, err := os.Create(baseFile)
		checkErr(err)
		defer file.Close()

		// 3. Copy image to the file
		_, err = io.Copy(file, resp.Body)
		checkErr(err)

		// 4. Change offset to 0 for the next read
		_, err = file.Seek(0, 0)
		checkErr(err)

		// 4. Read the image
		iii, err := jpeg.Decode(file)
		checkErr(err)

		// 5. Setup width and height from the request params
		width, err := strconv.Atoi(img.width)
		checkErr(err)
		height, err := strconv.Atoi(img.height)
		checkErr(err)
		m := resize.Resize(uint(width), uint(height), iii, resize.Lanczos3)

		// 6. Create the new one image with requested size
		out, err := os.Create(reqFile)
		checkErr(err)
		defer out.Close()

		// 8. Save it to the file
		err = jpeg.Encode(out, m, nil)
		checkErr(err)

		_, err = out.Seek(0, 0)
		checkErr(err)

		_, err = io.Copy(w, out)
		checkErr(err)

		log.Info("Save the image")
		h.cache.Set(storage.Key(reqFile), "")
	} else {
		log.Info("the image in the cache")

		file, err := os.Open(reqFile)
		checkErr(err)

		_, err = io.Copy(w, file)
		checkErr(err)
	}

	log.Info("Done ...")
}

func (s *Server) setRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, id)))
	})
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.WithFields(logrus.Fields{
			"remote_addr": r.RemoteAddr,
			"request_id":  r.Context().Value(ctxKeyRequestID),
		})
		logger.Infof("Started %s %s", r.Method, r.RequestURI)
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		logger.Infof("Complited with %d %s in %v",
			rw.code,
			http.StatusText(rw.code),
			time.Now().Sub(start),
		)
	})
}

func checkErr(err error) {
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

func (s *Server) configureLogger() error {
	level, err := logrus.ParseLevel(s.Config.LogLevel)
	if err != nil {
		return err
	}

	s.logger.SetLevel(level)

	return nil
}
