package router

import (
	"database/sql"
	"gooner/auth"
	"gooner/db"

	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SignupHandler(pool *db.DBPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.FormValue("email")
		username := r.FormValue("username")
		password := r.FormValue("password")

		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			http.Error(w, "Error hashing password", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		if err := db.InsertUser(pool, ctx, email, username, hashedPassword); err != nil {
			http.Error(w, "Error inserting user into database", http.StatusInternalServerError)
			return
		}

		response := "User registered successfully!"
		w.Write([]byte(response))
	}
}

func LoginHandler(pool *db.DBPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.FormValue("email")
		password := r.FormValue("password")

		ctx := r.Context()
		readTx, err := pool.GetReadTx(ctx)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("Failed to begin transaction: %v", err)
			return
		}
		defer readTx.Rollback()

		query := `SELECT user_id, email, username, password FROM users WHERE email = ?`
		var user struct {
			UserID   string
			Email    string
			Username string
			Password string
		}

		err = readTx.QueryRowContext(ctx, query, email).Scan(
			&user.UserID,
			&user.Email,
			&user.Username,
			&user.Password,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("Error querying user: %v", err)
			return
		}

		if err = readTx.Commit(); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("Failed to commit transaction: %v", err)
			return
		}

		pepperPW := append([]byte(password), auth.Pepper...)
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), pepperPW)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		payload := auth.NewPayload(user.UserID)
		token, err := auth.SignPayload(auth.Secret, payload)
		if err != nil {
			http.Error(w, "Authentication error", http.StatusInternalServerError)
			log.Printf("Failed to create JWT: %v", err)
			return
		}

		cookie := &http.Cookie{
			Name:     "AuthToken",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(auth.DefaultExpirationJWT),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/home", http.StatusSeeOther)
	}
}
