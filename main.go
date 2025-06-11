package main

import (
    "gooner/auth"
    "gooner/db"
    "gooner/middleware"
    "gooner/router"

    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

// we extend an interface, and then override whatever methode we need
// need to pass a ref of ResWriter because we only get the new status code after
// a handler has returned. a bit annoying
type statusResponseWriter struct {
    http.ResponseWriter
    status       int
    bytesWritten int64
}

func (w *statusResponseWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(w.status)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
    n, err := w.ResponseWriter.Write(b)
    w.bytesWritten += int64(n)
    return n, err
}

// a handler just serves http. got it
// handlerfunc allows me to turn anything into a handler, okay
// handlefunc (no "r"!) allows me to define pattern + handler in one go
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        srw := &statusResponseWriter{
            ResponseWriter: w,
            status:         http.StatusOK,
        }

        start := time.Now()
        next.ServeHTTP(srw, r)
        duration := time.Since(start)

        log.Printf("[REQUEST] [%s %s] [%s] [Status: %d] [Duration: %v] [Bytes written: %d]", r.Method, r.URL.Path, r.Proto, srw.status, duration, srw.bytesWritten)
    })
}

// func SayMAIN(next http.Handler) http.Handler {
//     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//         log.Println("I am middleware coming from MAIN MUX!")
//         next.ServeHTTP(w, r)
//     })
// }

func Run(pool *db.DBPool, router *router.Router, port string, name string) {
	// BUG: timeouts make long running requests fail silently!
    server := &http.Server{
        Addr:    ":" + port,
        Handler: router,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      30 * time.Second, 
        IdleTimeout:       120 * time.Second,
        ReadHeaderTimeout: 5 * time.Second,
    }

    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        sig := <-signalChan
        router.Logger.Printf("Received signal: %s. Shutting down. Rip '%s' ... \n", sig, name)

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            router.Logger.Printf("Server shutdown error: %v", err)
        }

        if pool.ReadDB != nil {
            pool.ReadDB.Close()
        }
        if pool.WriteDB != nil {
            pool.WriteDB.Close()
        }

        router.Logger.Println("Graceful shutdown completed")
    }()

    router.Logger.Printf("%s is running on port %s\n", name, port)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        router.Logger.Fatalf("Error starting server: %v\n", err)
    }

    router.Logger.Println("Server stopped.")
}

func main() {

    mainMux := router.NewRouter("MAIN")

    DBPool, err := db.InitDB("users.db")
    if err != nil {
		mainMux.Logger.Printf("Could not init database: %s", err)
    }

    sessionConfig := middleware.SessionConfig{
        JWTSecret: []byte(auth.Secret),
        PublicPaths: map[string]bool{
        "/":           true,
        "/assets":     true,
        "/api/signup": true,
        "/api/login":  true,
        },
    }

    authAdapter := func(next http.Handler) http.Handler {
        return middleware.AuthMiddleware(next, sessionConfig)
    }

    mainMux.Use(Logger)
    mainMux.Use(authAdapter)
    mainMux.RegisterFileServer("./static", "./static/assets")

    apiMux := router.NewRouter("API")
	apiMux.Pool = DBPool

    apiMux.Handle("POST /signup", router.SignupHandler)
    apiMux.Handle("POST /login", router.LoginHandler)
    mainMux.Include(apiMux, "/api")

    Run(DBPool, mainMux, "8000", "Retardo")
}
