package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/Parquery/revproxyry/config"
	"github.com/Parquery/revproxyry/sigterm"
	"github.com/Parquery/revproxyry/auth"
)

type logWriter struct {
	out io.Writer
}

func (lw *logWriter) Write(bytes []byte) (int, error) {
	msg := fmt.Sprintf("revproxyry: %s: %s",
		time.Now().UTC().Format("2006-01-02T15:04:05.999Z"), string(bytes))

	return lw.out.Write([]byte(msg))
}

type fileServer struct {
	root   http.Dir
	logErr *log.Logger
}

func (fs *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//add prefix and clean
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	upath = path.Clean(upath)

	//path to file

	name := path.Join(string(fs.root), filepath.FromSlash(upath))

	//check if file exists
	f, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

	http.ServeFile(w, r, name)
}

func newFileServer(root http.Dir, logErr *log.Logger) (*fileServer, error) {
	if string(root) == "" {
		return nil, fmt.Errorf("unexpected empty root")
	}

	return &fileServer{root: root, logErr: logErr}, nil
}

type loggingHandler struct {
	logOut  *log.Logger
	logErr  *log.Logger
	prefix  string
	target  string
	handler http.Handler
}

type logMessage struct {
	Method         string `json:"method"`
	URL            string `json:"url"`
	RemoteAddr     string `json:"remote_addr"`
	Prefix         string `json:"prefix"`
	Target         string `json:"target"`
	Error          string `json:"error"`
	StatusCode     int    `json:"status_code"`
	RedirectionURL string `json:"redirection_url"`
}

func newMessage(req *http.Request) logMessage {
	return logMessage{
		Method:     req.Method,
		URL:        req.URL.String(),
		RemoteAddr: req.RemoteAddr}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (h *loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: 0}

	h.handler.ServeHTTP(lrw, req)

	msg := newMessage(req)
	msg.Prefix = h.prefix
	msg.Target = h.target
	msg.StatusCode = lrw.statusCode

	bb, err := json.Marshal(&msg)
	if err != nil {
		http.Error(w, "Failed to JSON-encode log message", http.StatusInternalServerError)
		h.logErr.Printf("Failed to JSON-encode log message %#v: %s", msg, err.Error())
		return
	}

	h.logOut.Printf("%s\n", string(bb))
}

type authHandler struct {
	auths   *auth.Auths
	logErr  *log.Logger
	handler http.Handler
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	username, passw, ok := req.BasicAuth()
	if !ok {
		msg := newMessage(req)
		msg.Error = "no Auth"
		msg.StatusCode = http.StatusUnauthorized

		bb, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, "Failed to JSON-encode log message", http.StatusInternalServerError)
			h.logErr.Printf("Failed to JSON-encode log message %#v: %s", msg, err.Error())
			return
		}

		h.logErr.Printf("%s\n", string(bb))

		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "No basic Auth provided", http.StatusUnauthorized)
		return
	}

	var rejectionMsg string
	var err error
	ok, rejectionMsg, err = h.auths.Authenticate(username, passw)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to authenticate the user: %s", username),
			http.StatusInternalServerError)
		h.logErr.Printf("Failed to authenticate the user %s: %s", username, err.Error())
		return
	}

	if !ok {
		msg := newMessage(req)
		msg.Error = fmt.Sprintf("Auth not accepted for the user %s: %s", username, rejectionMsg)
		msg.StatusCode = http.StatusUnauthorized

		bb, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, "Failed to JSON-encode log message", http.StatusInternalServerError)
			h.logErr.Printf("Failed to JSON-encode log message %#v: %s", msg, err.Error())
			return
		}

		h.logErr.Printf("%s\n", string(bb))

		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Provided basic Auth not accepted", http.StatusUnauthorized)

		return
	}

	h.handler.ServeHTTP(w, req)
}

type args struct {
	revproxyPath *string
	quiet        *bool
}

func setupRouter(cfg *config.Config, logOut *log.Logger, logErr *log.Logger) (http.Handler, error) {

	router := http.NewServeMux()

	handledRoot := false

	for _, route := range cfg.Routes {

		parsedURL, _ := url.ParseRequestURI(route.Target)

		var handler http.Handler

		switch {
		case strings.HasPrefix(route.Target, "/"):
			var err error
			handler, err = newFileServer(http.Dir(route.Target), logErr)
			if err != nil {
				return nil, err
			}

		case parsedURL != nil:
			handler = httputil.NewSingleHostReverseProxy(parsedURL)

		default:
			return nil, fmt.Errorf("does not know how to handle the Route: %s", route.Target)
		}

		handler = &loggingHandler{
			logOut:  logOut,
			logErr:  logErr,
			prefix:  route.Prefix,
			target:  route.Target,
			handler: handler}

		authMap := make(map[string]*config.Auth)
		for _, authID := range route.AuthIDs {
			authMap[authID] = cfg.Auths[authID]
		}

		auths, err := auth.New(authMap)
		if err != nil {
			return nil, err
		}

		if !auths.All {
			handler = &authHandler{
				auths:   auths,
				logErr:  logErr,
				handler: handler}
		}

		router.Handle(route.Prefix, http.StripPrefix(route.Prefix, handler))

		if route.Prefix == "/" {
			handledRoot = true
		}
	}

	if !handledRoot{
		router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			msg := newMessage(req)
			msg.Error = "not found"
			msg.StatusCode = http.StatusNotFound

			bb, err := json.Marshal(&msg)
			if err != nil {
				http.Error(w, "Failed to JSON-encode log message", http.StatusInternalServerError)
				logErr.Printf("Failed to JSON-encode log message %#v: %s", msg, err.Error())
				return
			}

			logErr.Printf("%s\n", string(bb))

			http.Error(w, "Not found", http.StatusNotFound)
			return
		})
	}

	return router, nil
}

func setupRedirectionRouter(httpsAddr string, logOut *log.Logger, logErr *log.Logger) (http.Handler, error) {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var prefix string
		if strings.HasPrefix(httpsAddr, ":") {
			parts := strings.Split(req.Host, ":")
			host := parts[0]

			prefix = fmt.Sprintf("https://%s%s", host, httpsAddr)
		} else {
			prefix = fmt.Sprintf("https://") + httpsAddr
		}

		newURL := prefix + req.RequestURI

		msg := newMessage(req)
		msg.RedirectionURL = newURL
		msg.StatusCode = http.StatusMovedPermanently

		bb, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, "Failed to JSON-encode log message", http.StatusInternalServerError)
			logErr.Printf("Failed to JSON-encode log message %#v: %s", msg, err.Error())
			return
		}

		logOut.Printf("%s\n", string(bb))
		http.Redirect(w, req, newURL, http.StatusMovedPermanently)
	})

	return router, nil
}

func setupServers(
	cfg *config.Config, logOut *log.Logger, logErr *log.Logger) (httpd *http.Server, httpsd *http.Server, err error) {

	// set up a router
	router, err := setupRouter(cfg, logOut, logErr)
	if err != nil {
		err = fmt.Errorf("failed to set up the router: %s", err.Error())
		return
	}

	if cfg.SslCertPath == "" && cfg.LetsencryptDir == "" {
		httpd = &http.Server{Handler: router}
	} else {
		var rediRouter http.Handler
		rediRouter, err = setupRedirectionRouter(cfg.HttpsAddress, logOut, logErr)
		if err != nil {
			err = fmt.Errorf("failed to set up the redirection router: %s", err.Error())
			return
		}

		switch {
		case cfg.SslCertPath != "":
			httpd = &http.Server{Handler: rediRouter}
			httpsd = &http.Server{Handler: router}

		case cfg.LetsencryptDir != "":
			logOut.Printf("Setting up Let's encrypt to the directory: %#v\n", cfg.LetsencryptDir)
			hostPolicy := func(ctx context.Context, host string) error {
				allowedHost := cfg.Domain
				if host == allowedHost {
					return nil
				}
				return fmt.Errorf("acme/autocert: only %s host is allowed, got: %#v", allowedHost, host)
			}

			mger := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: hostPolicy,
				Cache:      autocert.DirCache(cfg.LetsencryptDir),
			}

			httpd = &http.Server{Handler: mger.HTTPHandler(rediRouter)}

			httpsd = &http.Server{
				TLSConfig: &tls.Config{GetCertificate: mger.GetCertificate},
				Handler:   router}

			if cfg.SslCertPath != "" {
				err = fmt.Errorf("expected empty SSL cert path, but got: %#v", cfg.SslCertPath)
				return
			}

			if cfg.SslKeyPath != "" {
				err = fmt.Errorf("expected empty SSL key path, but got: %#v", cfg.SslKeyPath)
				return
			}

		default:
			err = fmt.Errorf("unhandled execution path for revproxy: %#v", cfg)
			return
		}
	}

	if httpsd != nil {
		httpsd.Addr = cfg.HttpsAddress
		httpsd.ReadHeaderTimeout = 60 * time.Second
		httpsd.ReadTimeout = 60 * time.Second
		httpsd.IdleTimeout = 60 * time.Second
	}

	httpd.Addr = cfg.HttpAddress
	httpd.ReadHeaderTimeout = 60 * time.Second
	httpd.ReadTimeout = 60 * time.Second
	httpd.IdleTimeout = 60 * time.Second

	return httpd, httpsd, nil
}

func run() int {
	var a args
	a.revproxyPath = flag.String("config_path", "",
		"Path to the file containing the JSON-encoded configuration")

	a.quiet = flag.Bool("quiet", false, "If set, outputs as little messages as possible")

	version := flag.Bool("version", false,
		"If set, outputs only the version to the standard output and exits immediately")

	flag.Parse()

	if *version {
		fmt.Println("1.0.7")
		return 0
	}

	var logOut *log.Logger
	if *a.quiet {
		logOut = log.New(ioutil.Discard, "", 0)
	} else {
		logOut = log.New(&logWriter{out: os.Stdout}, "", 0)
	}

	logErr := log.New(&logWriter{out: os.Stderr}, "", 0)

	if *a.revproxyPath == "" {
		logErr.Println("-revproxy_path is mandatory")
		flag.PrintDefaults()
		return 1
	}

	logOut.Println("Hi!")

	var err error

	revproxy, err := config.Load(*a.revproxyPath)
	if err != nil {
		logErr.Printf("Failed to load the revproxy config from %s: %s\n", *a.revproxyPath, err.Error())
		return 1
	}

	err = config.Validate(revproxy)
	if err != nil {
		logErr.Printf("Validation of arguments and the revproxy specification failed: %s\n", err.Error())
		return 1
	}

	httpd, httpsd, err := setupServers(revproxy, logOut, logErr)
	if err != nil {
		logErr.Printf("Failed to set up the servers: %s\n", err.Error())
		return 1
	}

	failures := int32(0)  // atomic variable, increased on failures to start one of the servers
	var wg sync.WaitGroup // synchronizes printing of Route tables

	wg.Add(1)
	go func() {
		defer wg.Done()

		logOut.Printf("Listening for HTTP requests on the address: %#v\n", revproxy.HttpAddress)

		err = httpd.ListenAndServe()
		if err != http.ErrServerClosed {
			logErr.Printf("Failed to listen and serve on %s: %s\n", revproxy.HttpAddress, err.Error())
			atomic.AddInt32(&failures, 1)
		}
		logOut.Println("Goodbye from the http server.")
	}()

	if httpsd != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			logOut.Printf("Listening for HTTPS requests on the address: %#v\n", revproxy.HttpsAddress)

			err = httpsd.ListenAndServeTLS(revproxy.SslCertPath, revproxy.SslKeyPath)
			if err != http.ErrServerClosed {
				logErr.Printf("Failed to listen and serve on %s: %s\n", revproxy.HttpsAddress, err.Error())
				atomic.AddInt32(&failures, 1)
			}
			logOut.Println("Goodbye from the https server.")
		}()
	}

	sigterm.RegisterSIGTERMHandler()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for !sigterm.ReceivedSIGTERM() && atomic.LoadInt32(&failures) == 0 {
			time.Sleep(time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		httpd.Shutdown(ctx)

		if httpsd != nil {
			httpsd.Shutdown(ctx)
		}
	}()

	wg.Wait()

	logOut.Println("Goodbye from revproxyry.")

	return 0
}

func main() {
	os.Exit(run())
}
