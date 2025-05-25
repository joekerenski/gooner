package router

import (
    "database/sql"
    "gooner/auth"
    "gooner/db"
	"gooner/appcontext"

    "net/http"
    "time"

    "golang.org/x/crypto/bcrypt"
)

func SignupHandler(ctx *appcontext.AppContext) {
    email := ctx.Request.FormValue("email")
    username := ctx.Request.FormValue("username")
    password := ctx.Request.FormValue("password")

    hashedPassword, err := auth.HashPassword(password)
    if err != nil {
        http.Error(ctx.Writer, "Error hashing password", http.StatusInternalServerError)
        return
    }

    if err := db.InsertUser(ctx.Pool, ctx.Context, email, username, hashedPassword); err != nil {
        http.Error(ctx.Writer, "Error inserting user into database", http.StatusInternalServerError)
        return
    }

    response := "User registered successfully!"
    ctx.Writer.Write([]byte(response))
}

func LoginHandler(ctx *appcontext.AppContext) {
    email := ctx.Request.FormValue("email")
    password := ctx.Request.FormValue("password")

    readTx, err := ctx.Pool.GetReadTx(ctx.Context)
    if err != nil {
        http.Error(ctx.Writer, "Database error", http.StatusInternalServerError)
        ctx.Logger.Printf("Failed to begin transaction: %v", err)
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

    err = readTx.QueryRowContext(ctx.Context, query, email).Scan(
        &user.UserID,
        &user.Email,
        &user.Username,
        &user.Password,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(ctx.Writer, "Invalid credentials", http.StatusUnauthorized)
            return
        }
        http.Error(ctx.Writer, "Database error", http.StatusInternalServerError)
        ctx.Logger.Printf("Error querying user: %v", err)
        return
    }

    if err = readTx.Commit(); err != nil {
        http.Error(ctx.Writer, "Database error", http.StatusInternalServerError)
        ctx.Logger.Printf("Failed to commit transaction: %v", err)
        return
    }

    pepperPW := append([]byte(password), auth.Pepper...)
    err = bcrypt.CompareHashAndPassword([]byte(user.Password), pepperPW)
    if err != nil {
        http.Error(ctx.Writer, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    payload := auth.NewPayload(user.UserID)
    token, err := auth.SignPayload(auth.Secret, payload)
    if err != nil {
        http.Error(ctx.Writer, "Authentication error", http.StatusInternalServerError)
        ctx.Logger.Printf("Failed to create JWT: %v", err)
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
    http.SetCookie(ctx.Writer, cookie)
    http.Redirect(ctx.Writer, ctx.Request, "/home", http.StatusSeeOther)
}
