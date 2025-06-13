package db

import (
    "database/sql"
    "fmt"
	"time"
	"context"

	"gooner/auth"
)

func InsertUserSQLite(pool *DBPool, ctx context.Context, email string, username string, password string) error {

    writeTx, err := pool.GetWriteTx(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer writeTx.Rollback()

    query := `INSERT INTO users
    (user_id, email, username, created_at, password, sub_tier) VALUES
    (?, ?, ?, ?, ?, ?)`

    userId, err := GenUUID()
    if err != nil {
        return fmt.Errorf("failed to generate UUID: %w", err)
    }

    createdAt := time.Now()
    subTier := 0

    _, err = writeTx.Exec(query, userId, email, username, createdAt, password, subTier)
    if err != nil {
        return fmt.Errorf("failed to insert user: %w", err)
    }

    if err = writeTx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}

func GetAllUsersSQLite(pool *DBPool, ctx context.Context) ([]User, error) {

    readTx, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin read transaction: %w", err)
    }
    defer readTx.Rollback()

    query := `SELECT user_id, email, username, created_at, password, sub_tier
              FROM users ORDER BY created_at DESC`

    rows, err := readTx.QueryContext(ctx, query)
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

    if err = readTx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit read transaction: %w", err)
    }

    return users, nil
}

func GetUserByEmailSQLite(pool *DBPool, ctx context.Context, email string) (*User, error) {
    readTx, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin read transaction: %w", err)
    }
    defer readTx.Rollback()

    query := `SELECT user_id, email, username, created_at, password, sub_tier
              FROM users WHERE email = ?`

    var user User
    var createdAt time.Time

    err = readTx.QueryRowContext(ctx, query, email).Scan(
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

    if err = readTx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit read transaction: %w", err)
    }

    return &user, nil
}

// DB functions for refresh token flow
func StoreRefreshTokenSQLite(pool *DBPool, ctx context.Context, refreshToken auth.RefreshToken) error {
    writeTx, err := pool.GetWriteTx(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer writeTx.Rollback()

    tokenHash := auth.HashRefreshToken(refreshToken.Token)

    query := `INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at) 
              VALUES (?, ?, ?, ?)`

    _, err = writeTx.ExecContext(ctx, query, 
        tokenHash, 
        refreshToken.UserID, 
        refreshToken.ExpiresAt, 
        refreshToken.CreatedAt,
    )
    if err != nil {
        return fmt.Errorf("failed to store refresh token: %w", err)
    }

    return writeTx.Commit()
}

func GetValidRefreshTokenForUserSQLite(pool *DBPool, ctx context.Context, userID string) (*auth.RefreshToken, error) {
    readTx, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin read transaction: %w", err)
    }
    defer readTx.Rollback()

    query := `SELECT token_hash, expires_at, created_at 
              FROM refresh_tokens 
              WHERE user_id = ? AND expires_at > ? 
              ORDER BY created_at DESC 
              LIMIT 1`

    var tokenHash string
    var refreshToken auth.RefreshToken

    err = readTx.QueryRowContext(ctx, query, userID, time.Now()).Scan(
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

    return &refreshToken, readTx.Commit()
}

func RevokeRefreshTokensForUserSQLite(pool *DBPool, ctx context.Context, userID string) error {
    writeTx, err := pool.GetWriteTx(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer writeTx.Rollback()

    query := `DELETE FROM refresh_tokens WHERE user_id = ?`
    _, err = writeTx.ExecContext(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("failed to revoke refresh tokens: %w", err)
    }

    return writeTx.Commit()
}
