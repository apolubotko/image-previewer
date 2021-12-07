package proxy

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/apolubotko/image-previewer/internal/storage"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/nfnt/resize"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	imagePath          = "/tmp"
	fillPath           = "/fill"
	metricsPath        = "/metrics"
	sessionName        = "session"
	ctxKeyUser  ctxKey = iota
	ctxKeyRequestID
)

type ctxKey int8

var (
	errEmptyConfig = errors.New("empty config file")
)

type Server struct {
	Config     *Config
	cache      storage.Cache
	router     *mux.Router
	logger     *logrus.Logger
	httpClient *http.Client
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

	srv.Shutdown(ctx)

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
		var name, ext, height, width, reqUrl string
		var image *ImageObj

		tokens := strings.Split(r.URL.Path, "/")
		if len(tokens) > 4 {
			width = tokens[2]
			height = tokens[3]
			reqUrl = strings.Join(tokens[4:], "/")
			u, err := url.Parse(reqUrl)
			s.checkErr(err)
			if u.Scheme != "http" && u.Scheme != "https" {
				reqUrl = "http://" + reqUrl
			}
			image = &ImageObj{width: width, height: height, url: reqUrl}
			_ = image

			// 1. Step 1
			// gopher.jpg | name = gopher, ext = jpg
			base := path.Base(reqUrl)
			fileName := strings.Split(base, ".")
			if len(fileName) > 1 {
				name = fileName[0]
				ext = fileName[1]
			} else {
				s.respond(w, r, http.StatusNotAcceptable, errors.New("incorrect file extension"))
				return
			}
			// Step 2. Do request to requested image source
			resp, err := s.httpClient.Get(reqUrl)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				s.respond(w, r, http.StatusBadGateway, nil)
				return
			}
			if err != nil {
				s.error(w, r, http.StatusInternalServerError, err)
				return
			}
			// Path to requested file name like '/tmp/gopher_500x600.jpg'
			reqFile := imagePath + string(os.PathSeparator) + name + "_" + image.width + "x" + image.height + "." + ext
			// File with base image like '/tmp/gopher.jpg'
			baseFile := imagePath + string(os.PathSeparator) + base

			// 2. Check the cache
			s.logger.Info("Check the cache")
			_, ok := s.cache.Get(storage.Key(reqFile))
			if !ok {
				s.resizeImage(baseFile, resp.Body, width, height)
			} else {
				s.openFile()
			}

			w.WriteHeader(http.StatusOK)
		} else {
			s.logger.Info("Can't process request - ", r.URL.Path)

			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func (s *Server) resizeImage(baseFile string, srcFile io.ReadCloser, width, height string) {
	// 2. Create the base file on local disk
	file, err := os.Create(baseFile)
	s.checkErr(err)
	defer file.Close()

	// 3. Copy image to the file
	_, err = io.Copy(file, srcFile)
	s.checkErr(err)

	// 4. Change offset to 0 for the next read
	_, err = file.Seek(0, 0)
	s.checkErr(err)

	// 4. Read the image
	iii, err := jpeg.Decode(file)
	s.checkErr(err)

	// 5. Setup width and height from the request params
	w, err := strconv.Atoi(width)
	s.checkErr(err)
	h, err := strconv.Atoi(height)
	s.checkErr(err)
	m := resize.Resize(uint(w), uint(h), iii, resize.Lanczos3)

	// 6. Create the new one image with requested size
	out, err := os.Create(reqFile)
	s.checkErr(err)
	defer out.Close()
}

// http://localhost:8088/img/gopher.jpg
func (s *Server) processFillRequest(w http.ResponseWriter, r *http.Request, img *ImageObj) {
	var name, ext string

	// 1. Step 1
	// gopher.jpg | name = gopher, ext = jpg
	base := path.Base(img.url)
	fileName := strings.Split(base, ".")
	if len(fileName) > 1 {
		name = fileName[0]
		ext = fileName[1]
	}
	// resp, err := http.Get(img.url)
	resp, err := s.httpClient.Get(img.url)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.checkErr(err)
	defer resp.Body.Close()

	// Path to requested file name like '/tmp/gopher_500x600.jpg'
	reqFile := imagePath + string(os.PathSeparator) + name + "_" + img.width + "x" + img.height + "." + ext
	// File with base image like '/tmp/gopher.jpg'
	baseFile := imagePath + string(os.PathSeparator) + base

	// 2. Check the cache
	s.logger.Info("Check the cache")
	_, ok := s.cache.Get(storage.Key(reqFile))

	if !ok {
		s.logger.Info("File not found in local cache. Creating ...")

		// 2. Create the base file on local disk
		file, err := os.Create(baseFile)
		s.checkErr(err)
		defer file.Close()

		// 3. Copy image to the file
		_, err = io.Copy(file, resp.Body)
		s.checkErr(err)

		// 4. Change offset to 0 for the next read
		_, err = file.Seek(0, 0)
		s.checkErr(err)

		// 4. Read the image
		iii, err := jpeg.Decode(file)
		s.checkErr(err)

		// 5. Setup width and height from the request params
		width, err := strconv.Atoi(img.width)
		s.checkErr(err)
		height, err := strconv.Atoi(img.height)
		s.checkErr(err)
		m := resize.Resize(uint(width), uint(height), iii, resize.Lanczos3)

		// 6. Create the new one image with requested size
		out, err := os.Create(reqFile)
		s.checkErr(err)
		defer out.Close()

		// 8. Save it to the file
		err = jpeg.Encode(out, m, nil)
		s.checkErr(err)

		_, err = out.Seek(0, 0)
		s.checkErr(err)

		_, err = io.Copy(w, out)
		s.checkErr(err)

		s.logger.Info("Save the image")
		s.cache.Set(storage.Key(reqFile), "")
	} else {
		s.logger.Info("the image in the cache")

		file, err := os.Open(reqFile)
		s.checkErr(err)

		_, err = io.Copy(w, file)
		s.checkErr(err)
	}

	s.logger.Info("Done ...")
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
		json.NewEncoder(w).Encode(data)
	}
}
