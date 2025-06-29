package router

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"io"

	"gooner/appcontext"
	"gooner/db"
)

type AppHandlerFunc func(ctx *appcontext.AppContext)

type Router struct {
    mux        *http.ServeMux
    mw         []func(http.Handler) http.Handler
    handler    http.Handler
    tag        string
    Pool       *db.DBPool
    Logger     *log.Logger
    HTTPClient *http.Client
}

func NewRouter(tag string) *Router {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 20,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
        DisableKeepAlives:   false,
        DisableCompression:  false,
        MaxConnsPerHost:     0,
        ResponseHeaderTimeout: 30 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    }

    client := &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }

    return &Router{
        mux:        http.NewServeMux(),
        mw:         []func(http.Handler) http.Handler{},
        handler:    nil,
        tag:        tag,
        Pool:       nil,
        Logger:     log.New(os.Stdout, "["+tag+"] ", log.LstdFlags),
        HTTPClient: client,
    }
}

func (m *Router) applyMiddleware() {
	handler := http.Handler(m.mux)
	for i := len(m.mw) - 1; i >= 0; i-- {
		handler = m.mw[i](handler)
	}
	m.handler = handler
}

func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.handler == nil {
		m.applyMiddleware()
	}
	m.handler.ServeHTTP(w, r)
}

func (m *Router) Use(middleware func(http.Handler) http.Handler) {
	m.mw = append(m.mw, middleware)
	m.handler = nil
}

func (m *Router) Handle(pattern string, handler AppHandlerFunc) {
    m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
        ctx := appcontext.GetAppContext()
        ctx.Writer = w
        ctx.Request = r
        ctx.Context = r.Context()
        ctx.Logger = m.Logger
        ctx.Pool = m.Pool

        defer func() {
            if r.Body != nil {
                io.Copy(io.Discard, r.Body)
                r.Body.Close()
            }
            appcontext.CleanPut(ctx)
        }()

        handler(ctx)
    })
}

func (m *Router) HandleStatic(pattern string, handler http.Handler) {
    wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if r.Body != nil {
                io.Copy(io.Discard, r.Body)
                r.Body.Close()
            }
        }()
        handler.ServeHTTP(w, r)
    })
    m.mux.Handle(pattern, wrappedHandler)
}

func (m *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
    m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if r.Body != nil {
                io.Copy(io.Discard, r.Body)
                r.Body.Close()
            }
        }()
        handler(w, r)
    })
}

func (m *Router) Include(router *Router, prefix string) {
	if router.handler == nil {
		router.applyMiddleware()
	}
	m.mux.Handle(prefix+"/", http.StripPrefix(prefix, router.handler))
}

// TODO: optimize this
func (m *Router) RegisterFileServer(htmlPath string, assets string) {
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(htmlPath, "index.html"))
			return
		}
		http.NotFound(w, r)
	})

	err := filepath.Walk(htmlPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			relPath, err := filepath.Rel(htmlPath, path)
			if err != nil {
				return err
			}
			urlPath := "/" + strings.TrimSuffix(relPath, ".html")

			m.HandleFunc(urlPath, func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, path)
			})
			m.Logger.Printf("Registering %s -> %s", relPath, urlPath)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", htmlPath, err)
	}

	fs := http.FileServer(http.Dir(assets))
	m.HandleStatic("/assets/", http.StripPrefix("/assets/", fs))
}
