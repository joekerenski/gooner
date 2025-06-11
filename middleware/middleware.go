package middleware

import (
    "context"
    "net/http"
    "strings"
    "time"
	"log"

    "gooner/appcontext"
    "gooner/auth"
    "gooner/db"
)

type SessionConfig struct {
    JWTSecret   []byte
    PublicPaths map[string]bool
}

func AuthMiddleware(next http.Handler, config SessionConfig) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        appCtx := appcontext.GetAppContext()
        appCtx.Writer = w
        appCtx.Request = r
        appCtx.Context = r.Context()
        defer appcontext.CleanPut(appCtx)

        if config.PublicPaths[r.URL.Path] || strings.HasPrefix(r.URL.Path, "/assets/") {
            next.ServeHTTP(w, r)
            return
        }

        jwtCookie, err := appCtx.Request.Cookie("AuthToken")
        if err != nil || jwtCookie.Value == "" {
            redirectToLogin(appCtx)
            return
        }

        payload, err := auth.VerifyPayload(string(config.JWTSecret), jwtCookie.Value)
        if err != nil {
            if handleTokenRefresh(appCtx, config, jwtCookie.Value) {
                next.ServeHTTP(w, r)
                return
            }
            redirectToLogin(appCtx)
            return
        }

        appCtx.Context = context.WithValue(appCtx.Context, "userID", payload.Sub)
        r = r.WithContext(appCtx.Context)
        next.ServeHTTP(w, r)
    })
}

func handleTokenRefresh(appCtx *appcontext.AppContext, config SessionConfig, expiredJWT string) bool {
    userID, err := auth.ExtractUserIDFromExpiredJWT(expiredJWT)
    if err != nil {
        appCtx.Logger.Printf("Failed to extract user ID from expired JWT: %v", err)
        return false
    }

    refreshToken, err := db.GetValidRefreshTokenForUser(appCtx.Pool, appCtx.Context, userID)
    if err != nil || refreshToken == nil {
        if err != nil {
            appCtx.Logger.Printf("Failed to get refresh token for user %s: %v", userID, err)
        }
        return false
    }

    newPayload := auth.NewPayload(userID)
    newJWT, err := auth.SignPayload(string(config.JWTSecret), newPayload)
    if err != nil {
        appCtx.Logger.Printf("Failed to sign new JWT for user %s: %v", userID, err)
        return false
    }

    jwtCookie := &http.Cookie{
        Name:     "AuthToken",
        Value:    newJWT,
        Path:     "/",
        Expires:  time.Now().Add(auth.DefaultJWTExpiration),
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    }
    http.SetCookie(appCtx.Writer, jwtCookie)

    appCtx.Context = context.WithValue(appCtx.Context, "userID", userID)
    *appCtx.Request = *appCtx.Request.WithContext(appCtx.Context)

    appCtx.Logger.Printf("Successfully refreshed JWT for user %s", userID)
    return true
}

func redirectToLogin(appCtx *appcontext.AppContext) {
    if isAPIRequest(appCtx.Request) {
        http.Error(appCtx.Writer, "Authentication required", http.StatusUnauthorized)
    } else {
        http.Redirect(appCtx.Writer, appCtx.Request, "/", http.StatusSeeOther)
    }
}

func isAPIRequest(r *http.Request) bool {
    return strings.HasPrefix(r.URL.Path, "/api/") || 
           r.Header.Get("Content-Type") == "application/json" ||
           r.Header.Get("Accept") == "application/json"
}

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
