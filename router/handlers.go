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
        ctx.Logger.Printf("User insert error: %v", err)
        return
    }

    response := "User registered successfully!"
    ctx.Writer.Write([]byte(response))
}

func LoginHandler(ctx *appcontext.AppContext) {
    email := ctx.Request.FormValue("email")
    password := ctx.Request.FormValue("password")

	user, err := db.GetUserByEmail(ctx.Pool, ctx.Context, email)
    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(ctx.Writer, "Invalid credentials", http.StatusUnauthorized)
            return
        }
        http.Error(ctx.Writer, "Database error", http.StatusInternalServerError)
        ctx.Logger.Printf("Error querying user: %v", err)
        return
	}

    pepperPW := append([]byte(password), auth.Pepper...)
    err = bcrypt.CompareHashAndPassword([]byte(user.Password), pepperPW)
    if err != nil {
        http.Error(ctx.Writer, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    payload := auth.NewPayload(user.Id)
    jwtToken, err := auth.SignPayload(auth.JWTSecret, payload)
    if err != nil {
        http.Error(ctx.Writer, "Authentication error", http.StatusInternalServerError)
        ctx.Logger.Printf("Failed to create JWT: %v", err)
        return
    }

    refreshToken := auth.NewRefreshToken(user.Id)
    err = db.StoreRefreshToken(ctx.Pool, ctx.Context, refreshToken)
    if err != nil {
        http.Error(ctx.Writer, "Authentication error", http.StatusInternalServerError)
        ctx.Logger.Printf("Failed to store refresh token: %v", err)
        return
    }

    jwtCookie := &http.Cookie{
        Name:     "AuthToken",
        Value:    jwtToken,
        Path:     "/",
        Expires:  time.Now().Add(auth.JWTExpiration),
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    }
    http.SetCookie(ctx.Writer, jwtCookie)

    http.Redirect(ctx.Writer, ctx.Request, "/home", http.StatusSeeOther)
}

func LogoutHandler(ctx *appcontext.AppContext) {
    userID, ok := ctx.Context.Value("userID").(string)  // UserContextKey
    if ok && userID != "" {
        db.RevokeRefreshTokensForUser(ctx.Pool, ctx.Context, userID)
    }

    jwtCookie := &http.Cookie{
        Name:     "AuthToken",
        Value:    "",
        Path:     "/",
        Expires:  time.Unix(0, 0),
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    }
    http.SetCookie(ctx.Writer, jwtCookie)

    http.Redirect(ctx.Writer, ctx.Request, "/", http.StatusSeeOther)
}

