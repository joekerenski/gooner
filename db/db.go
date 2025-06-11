package db

import (
    "database/sql"
    "fmt"
	"time"
	"context"
	"runtime"
	"math/rand"
	"net/url"

	"gooner/auth"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    _ "github.com/mattn/go-sqlite3"
)

type User struct {
    Id        string    `json:"id"`
    Email     string    `json:"email"`
    UserName  string    `json:"username"`
    CreatedAt time.Time `json:"created_at"`
    Password  string    `json:"-"`
    SubTier   int       `json:"sub_tier"`
}

func InitDB(database string) (*DBPool, error) {
    writeDB, err := openConnection(database, false)
    if err != nil {
        return nil, fmt.Errorf("write pool init failed: %w", err)
    }

    readDB, err := openConnection(database, true)
    if err != nil {
        return nil, fmt.Errorf("read pool init failed: %w", err)
    }

    if err := runMigrations(writeDB); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    return &DBPool{
        ReadDB:  readDB,
        WriteDB: writeDB,
    }, nil
}

func runMigrations(db *sql.DB) error {
    driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
    if err != nil {
        return err
    }

    m, err := migrate.NewWithDatabaseInstance("file://migrations", "sqlite3", driver)
    if err != nil {
        return err
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return err
    }

    return nil
}

type RequestDB struct {
    *sql.Tx
    conn *sql.DB
}

type DBPool struct {
    ReadDB  *sql.DB
    WriteDB *sql.DB
}

const (
    busyTimeout = "5000"      // 5 seconds
    cacheSize   = "-20000"    // 20MB
    mmapSize    = "268435456" // 256MB
    journalMode = "WAL"
    synchronous = "NORMAL"
    tempStore   = "MEMORY"
    foreignKeys = "true"
)


func openConnection(database string, readonly bool) (*sql.DB, error) {
    params := make(url.Values)
    params.Add("_journal_mode", journalMode)
    params.Add("_busy_timeout", busyTimeout)
    params.Add("_synchronous", synchronous)
    params.Add("_cache_size", cacheSize)
    params.Add("_foreign_keys", foreignKeys)
    params.Add("_temp_store", tempStore)

    if readonly {
        params.Add("mode", "ro")
    } else {
        params.Add("mode", "rwc")
        params.Add("_txlock", "immediate")
    }

    connStr := fmt.Sprintf("file:%s?%s", database, params.Encode())
    db, err := sql.Open("sqlite3", connStr)
    if err != nil {
        return nil, err
    }

    _, err = db.Exec(fmt.Sprintf("PRAGMA mmap_size=%s;", mmapSize))
    if err != nil {
        return nil, fmt.Errorf("mmap_size pragma failed: %w", err)
    }

    if readonly {
        db.SetMaxOpenConns(max(2, runtime.NumCPU()))
        db.SetMaxIdleConns(2)
    } else {
        db.SetMaxOpenConns(1)
        db.SetMaxIdleConns(1)
    }

    db.SetConnMaxLifetime(time.Hour)

    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("connection ping failed: %w", err)
    }

    return db, nil
}

func (pool *DBPool) GetReadTx(ctx context.Context) (*RequestDB, error) {
    tx, err := pool.ReadDB.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelReadCommitted,
    })
    if err != nil {
        return nil, err
    }
    return &RequestDB{Tx: tx, conn: pool.ReadDB}, nil
}

func (pool *DBPool) GetWriteTx(ctx context.Context) (*RequestDB, error) {
    tx, err := pool.WriteDB.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable,
    })
    if err != nil {
        return nil, err
    }
    return &RequestDB{Tx: tx, conn: pool.WriteDB}, nil
}

func GenUUID() (string, error) {
    uuidBytes := make([]byte, 16)
    _, err := rand.Read(uuidBytes)
    if err != nil {
        return "", err
    }

    uuidBytes[6] = (uuidBytes[6] & 0x0f) | 0x40
    uuidBytes[8] = (uuidBytes[8] & 0x3f) | 0x80

    uuidStr := fmt.Sprintf("%x-%x-%x-%x-%x",
        uuidBytes[0:4],
        uuidBytes[4:6],
        uuidBytes[6:8],
        uuidBytes[8:10],
        uuidBytes[10:16])

    return uuidStr, nil
}

func (rdb *RequestDB) Commit() error {
    return rdb.Tx.Commit()
}

func (rdb *RequestDB) Rollback() error {
    return rdb.Tx.Rollback()
}

func InsertUser(pool *DBPool, ctx context.Context, email string, username string, password string) error {

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

func GetAllUsers(pool *DBPool, ctx context.Context) ([]User, error) {

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

func GetUserByEmail(pool *DBPool, ctx context.Context, email string) (*User, error) {
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
func StoreRefreshToken(pool *DBPool, ctx context.Context, refreshToken auth.RefreshToken) error {
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

func GetValidRefreshTokenForUser(pool *DBPool, ctx context.Context, userID string) (*auth.RefreshToken, error) {
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

func RevokeRefreshTokensForUser(pool *DBPool, ctx context.Context, userID string) error {
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
