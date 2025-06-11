package main

import (
    "gooner/db"
    "gooner/middleware"
    "gooner/router"
    "gooner/auth"

	"gooner/chat"

    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

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

    mainMux.Use(middleware.Logger)
    mainMux.Use(authAdapter)
    mainMux.RegisterFileServer("./static", "./static/assets")

    apiMux := router.NewRouter("API")
	apiMux.Pool = DBPool

    apiMux.Handle("POST /signup", router.SignupHandler)
    apiMux.Handle("POST /login", router.LoginHandler)

	apiMux.Handle("POST /chat/send", chat.SendMessageHandler)
	apiMux.Handle("GET /chat/messages", chat.GetMessagesHandler)
	apiMux.Handle("GET /stress-test", chat.StressTestHandler)

    mainMux.Include(apiMux, "/api")

    Run(DBPool, mainMux, "8000", "Retardo")
}
