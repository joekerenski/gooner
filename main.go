package main

import (
    "gooner/db"
    "gooner/middleware"
    "gooner/router"
	"gooner/auth"
	"gooner/webhooks"
	"gooner/config"
	"gooner/websocket"
	"gooner/admin"

	"gooner/chat"

    "context"
    "net/http"
    "os"
	"log"
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
    config, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    dbConfig := db.DatabaseConfig{
        Type:     config.Database.Type,
        Database: config.Database.Database,
        Host:     config.Database.Host,
        Port:     config.Database.Port,
        User:     config.Database.User,
        Password: config.Database.Password,
        SSLMode:  config.Database.SSLMode,
    }

    jwtExp, _ := time.ParseDuration(config.Auth.TokenExpiry)
    refreshExp, _ := time.ParseDuration(config.Auth.RefreshExpiry)

    auth.InitAuthParams(
        config.Auth.JWTSecret,
        config.Auth.RefreshSecret,
        config.Auth.Pepper,
        jwtExp,
        refreshExp,
    )
	
	wsHub := websocket.NewHub()
    go wsHub.Run()

    mainMux := router.NewRouter(config.Server.Name)

    DBPool, err := db.InitDB(dbConfig)
    if err != nil {
		mainMux.Logger.Printf("Could not init database: %s", err)
    }

    sessionConfig := middleware.SessionConfig{
        JWTSecret: []byte(config.Auth.JWTSecret),
        PublicPaths: map[string]bool{
            "/":               true,
            "/assets":         true,
            "/api/signup":     true,
            "/api/login":      true,
            "/api/webhooks":   true,
        },
    }

    authAdapter := func(next http.Handler) http.Handler {
        return middleware.AuthMiddleware(next, sessionConfig)
    }

    webhookHandler := webhooks.NewWebhookHandler(config.Webhooks.Secret)

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
    apiMux.Handle("POST /webhooks/generic", webhookHandler.GenericWebhook)
    apiMux.Handle("GET /ws", websocket.WebSocketHandler(wsHub))

	apiMux.Handle("GET /admin/metrics", admin.MetricsHandler)

    mainMux.Include(apiMux, "/api")

    Run(DBPool, mainMux, config.Server.Port, config.Server.Name)
}
