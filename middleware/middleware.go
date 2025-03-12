package middleware

import (
	"context"
	"database/sql"
	"gooner/auth"
	"gooner/db"
	"log"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
	DBContextKey   contextKey = "db"
)

type SessionConfig struct {
	JWTSecret   []byte
	DBPool      *db.DBPool
	PublicPaths map[string]bool
}

func AuthMiddleware(next http.Handler, config SessionConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), DBContextKey, config.DBPool)

		path := r.URL.Path

        for publicPath, allowed := range config.PublicPaths {
            if allowed && (path == publicPath || strings.HasPrefix(path, publicPath+"/")) {
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
        }

		cookie, err := r.Cookie("AuthToken")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// TODO: accept bytes for the jwt secret
		// also add expiry check here -> redirect to login
		payload, err := auth.VerifyPayload(string(config.JWTSecret), cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		readTx, err := config.DBPool.GetReadTx(ctx)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			log.Printf("Failed to get read transaction: %v", err)
			return
		}
		defer readTx.Rollback()

		var user db.User
		query := `SELECT user_id, email, username, created_at, password, sub_tier
                  FROM users WHERE user_id = ?`

		err = readTx.QueryRowContext(ctx, query, payload.Sub).Scan(
			&user.Id,
			&user.Email,
			&user.UserName,
			&user.CreatedAt,
			&user.Password,
			&user.SubTier,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Unauthorized: User not found", http.StatusUnauthorized)
			} else {
				log.Printf("Database error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		if err = readTx.Commit(); err != nil {
			log.Printf("Failed to commit read transaction: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctx = context.WithValue(ctx, UserContextKey, user)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
