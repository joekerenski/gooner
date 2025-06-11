package middleware

import (
    "context"
    "net/http"
    "strings"
    "time"

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
        http.Redirect(appCtx.Writer, appCtx.Request, "/login", http.StatusSeeOther)
    }
}

func isAPIRequest(r *http.Request) bool {
    return strings.HasPrefix(r.URL.Path, "/api/") || 
           r.Header.Get("Content-Type") == "application/json" ||
           r.Header.Get("Accept") == "application/json"
}

