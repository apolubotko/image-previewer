package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/nfnt/resize"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sirupsen/logrus"
)

const (
	imagePath              = "/tmp"
	fillPath               = "/fill"
	metricsPath            = "/metrics"
	ctxKeyRequestID ctxKey = iota
	minPartsLen            = 5
	weightIdx              = 2
	heightIdx              = 3
	urlIdx                 = 4
)

type ctxKey int8

var (
	errEmptyConfig = errors.New("empty config file")
	cacheSize      = promauto.NewCounter(prometheus.CounterOpts{
		Name: "image_previever_cache_size",
		Help: "The total number of cache elements",
	})
)

type Cache interface {
	Set(key Key, value interface{}) bool
	Get(key Key) (interface{}, bool)
	Clear()
}

type Server struct {
	Config     *Config
	cache      Cache
	router     *mux.Router
	logger     *logrus.Logger
	httpClient *http.Client
}

type ImageObj struct {
	name   string
	ext    string
	base   string
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
	server.cache = NewCache(config.CacheSize)
	server.httpClient = &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxConnsPerHost:     100,
			MaxIdleConnsPerHost: 100,
		},
	}

	return server, nil
}

func (s *Server) Start() {
	if err := s.configureLogger(); err != nil {
		s.logger.Fatal(err)
	}

	s.configureRouter()

	s.logger.Info("Starting Proxy Server ...")

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

	err := srv.Shutdown(ctx)
	s.checkErr(err)

	s.logger.Info("shutting down")
	os.Exit(0)
}

func (s *Server) configureRouter() {
	s.router.Use(s.setRequestID)
	s.router.Use(s.logRequest)
	s.router.Use(handlers.CORS(handlers.AllowedOrigins([]string{"*"})))
	s.router.PathPrefix(fillPath).HandlerFunc(s.handleFilRequest())
	s.router.PathPrefix(metricsPath).Handler(promhttp.Handler())

	//*********************************************************//
	//														   //
	//		      Add here your handlers and endpoints   	   //
	//														   //
	// Example:												   //
	// 	s.router.HandleFunc("/user", s.handleUserCreate()).    //
	//	Methods(http.MethodPost)							   //
	//														   //
	//  OR							   						   //
	//	s.router.PathPrefix(fillPath).						   //
	//    HandlerFunc(s.handleFilRequest())					   //
	//*********************************************************//
}

// /fill/50/50/localhost:8088/img/gopher.jpg
func (s *Server) handleFilRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var baseFile, reqFile string
		var image *image.Image
		var out *os.File
		defer out.Close()
		log := s.logger

		// Step 1. Create ImageObject to process request
		imgObj, err := generateImageObject(r.URL.Path)
		log.Info("Generate the ImageObject")
		s.checkErr(err)

		// Step 2. Do request to requested image source
		resp, err := s.httpClient.Get(imgObj.url)
		log.Info("Do client request")
		if err != nil {
			s.error(w, r, http.StatusNotFound, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			s.respond(w, r, http.StatusBadGateway, nil)
			return
		}
		baseFile, reqFile = s.createFileName(imgObj)

		// Step 3. Check the cache
		log.Info("Check the cache")
		if _, ok := s.cache.Get(Key(reqFile)); !ok {
			image = s.resizeImage(baseFile, resp.Body, imgObj)
			out = s.saveOnDisk(image, reqFile)
			s.cache.Set(Key(reqFile), imgObj)
			cacheSize.Inc()
		} else {
			out, err = os.Open(reqFile)
			s.checkErr(err)
		}
		// Step 4. Write the response with resized image
		log.Info("Send response")
		_, err = io.Copy(w, out)
		s.checkErr(err)
		w.WriteHeader(http.StatusOK)
	}
}

func generateImageObject(urlPath string) (*ImageObj, error) {
	imgObj := &ImageObj{}

	tokens := strings.Split(urlPath, "/")
	// Step 1. Check the num of tokens in url
	if len(tokens) < minPartsLen {
		e := fmt.Sprintf("Can't process request - %s", urlPath)
		return nil, errors.New(e)
	}
	// Step 2. Create the img object and set the struct fields
	imgObj.width = tokens[weightIdx]
	imgObj.height = tokens[heightIdx]
	imgObj.url = strings.Join(tokens[urlIdx:], "/")

	u, err := url.Parse(imgObj.url)
	if err != nil {
		return nil, errors.New("not enought tokens in url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		imgObj.url = "http://" + imgObj.url
	}
	imgObj.base = path.Base(imgObj.url)
	fileName := strings.Split(imgObj.base, ".")
	if len(fileName) <= 1 {
		return nil, errors.New("incorrect file extension")
	}
	imgObj.name = fileName[0]
	imgObj.ext = fileName[1]

	return imgObj, nil
}

func (s *Server) resizeImage(baseFile string, srcFile io.ReadCloser, imgObj *ImageObj) *image.Image {
	// 1. Create the base file on local disk
	file, err := os.Create(baseFile)
	s.checkErr(err)
	defer file.Close()

	// 2. Copy image to the file
	_, err = io.Copy(file, srcFile)
	s.checkErr(err)

	// 3. Change offset to 0 for the next read
	_, err = file.Seek(0, 0)
	s.checkErr(err)

	// 4. Read the image
	iii, err := jpeg.Decode(file)
	s.checkErr(err)

	// 5. Setup width and height from the request params
	w, err := strconv.Atoi(imgObj.width)
	s.checkErr(err)
	h, err := strconv.Atoi(imgObj.height)
	s.checkErr(err)
	m := resize.Resize(uint(w), uint(h), iii, resize.Lanczos3)

	return &m
}

func (s *Server) saveOnDisk(img *image.Image, reqFile string) *os.File {
	// 6. Create the new one image with requested size
	out, err := os.Create(reqFile)
	s.checkErr(err)

	// 7. Save it to the file
	err = jpeg.Encode(out, *img, nil)
	s.checkErr(err)

	_, err = out.Seek(0, 0)
	s.checkErr(err)

	return out
}

func (s *Server) createFileName(imgObj *ImageObj) (string, string) {
	// Path to requested file name like '/tmp/gopher_500x600.jpg'
	reqFile := imagePath + string(os.PathSeparator) + imgObj.name + "_" + imgObj.width + "x" + imgObj.height + "." + imgObj.ext
	// File with base image like '/tmp/gopher.jpg'
	baseFile := imagePath + string(os.PathSeparator) + imgObj.base

	return baseFile, reqFile
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

		logger.Infof("Completed with %d %s in %v",
			rw.code,
			http.StatusText(rw.code),
			time.Since(start),
		)
	})
}

func (s *Server) checkErr(err error) {
	if err != nil {
		s.logger.Fatalf("Error: %v\n", err)
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

func (s *Server) error(w http.ResponseWriter, r *http.Request, code int, err error) {
	s.respond(w, r, code, map[string]string{"error": err.Error()})
}

func (s *Server) respond(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
	w.WriteHeader(code)
	if data != nil {
		err := json.NewEncoder(w).Encode(data)
		s.checkErr(err)

	}
}
