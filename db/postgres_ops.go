package db

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    "gooner/auth"
)

func InsertUserPG(pool *DBPool, ctx context.Context, email string, username string, password string) error {
    tx, err := pool.PgxPool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    query := `INSERT INTO users
    (user_id, email, username, created_at, password, sub_tier) VALUES
    ($1, $2, $3, $4, $5, $6)`

    userId, err := GenUUID()
    if err != nil {
        return fmt.Errorf("failed to generate UUID: %w", err)
    }

    createdAt := time.Now()
    subTier := 0

    _, err = tx.Exec(ctx, query, userId, email, username, createdAt, password, subTier)
    if err != nil {
        return fmt.Errorf("failed to insert user: %w", err)
    }

    if err = tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}

func GetAllUsersPG(pool *DBPool, ctx context.Context) ([]User, error) {
    query := `SELECT user_id, email, username, created_at, password, sub_tier
              FROM users ORDER BY created_at DESC`

    rows, err := pool.PgxPool.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to query users: %w", err)
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var user User
        var createdAt time.Time

        err := rows.Scan(
            &user.Id,
            &user.Email,
            &user.UserName,
            &createdAt,
            &user.Password,
            &user.SubTier,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan user row: %w", err)
        }

        user.CreatedAt = createdAt
        users = append(users, user)
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating user rows: %w", err)
    }

    return users, nil
}

func GetUserByEmailPG(pool *DBPool, ctx context.Context, email string) (*User, error) {
    query := `SELECT user_id, email, username, created_at, password, sub_tier
              FROM users WHERE email = $1`

    var user User
    var createdAt time.Time

    err := pool.PgxPool.QueryRow(ctx, query, email).Scan(
        &user.Id,
        &user.Email,
        &user.UserName,
        &createdAt,
        &user.Password,
        &user.SubTier,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, fmt.Errorf("failed to query user by email: %w", err)
    }

    user.CreatedAt = createdAt
    return &user, nil
}

func StoreRefreshTokenPG(pool *DBPool, ctx context.Context, refreshToken auth.RefreshToken) error {
    tx, err := pool.PgxPool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    tokenHash := auth.HashRefreshToken(refreshToken.Token)

    query := `INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at) 
              VALUES ($1, $2, $3, $4)`

    _, err = tx.Exec(ctx, query, 
        tokenHash, 
        refreshToken.UserID, 
        refreshToken.ExpiresAt, 
        refreshToken.CreatedAt,
    )
    if err != nil {
        return fmt.Errorf("failed to store refresh token: %w", err)
    }

    return tx.Commit(ctx)
}

func GetValidRefreshTokenForUserPG(pool *DBPool, ctx context.Context, userID string) (*auth.RefreshToken, error) {
    query := `SELECT token_hash, expires_at, created_at 
              FROM refresh_tokens 
              WHERE user_id = $1 AND expires_at > $2 
              ORDER BY created_at DESC 
              LIMIT 1`

    var tokenHash string
    var refreshToken auth.RefreshToken

    err := pool.PgxPool.QueryRow(ctx, query, userID, time.Now()).Scan(
        &tokenHash,
        &refreshToken.ExpiresAt,
        &refreshToken.CreatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, fmt.Errorf("failed to get refresh token: %w", err)
    }

    refreshToken.UserID = userID
    return &refreshToken, nil
}

func RevokeRefreshTokensForUserPG(pool *DBPool, ctx context.Context, userID string) error {
    tx, err := pool.PgxPool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    query := `DELETE FROM refresh_tokens WHERE user_id = $1`
    _, err = tx.Exec(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("failed to revoke refresh tokens: %w", err)
    }

    return tx.Commit(ctx)
}

