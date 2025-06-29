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
    "github.com/golang-migrate/migrate/v4/database"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/mattn/go-sqlite3"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/jackc/pgx/v5/stdlib"
)

type DatabaseConfig struct {
    Type     string
    Database string
    Host     string
    Port     int
    User     string
    Password string
    SSLMode  string
}

type User struct {
    Id        string    `json:"id"`
    Email     string    `json:"email"`
    UserName  string    `json:"username"`
    CreatedAt time.Time `json:"created_at"`
    Password  string    `json:"-"`
    SubTier   int       `json:"sub_tier"`
}

func InitDB(config DatabaseConfig) (*DBPool, error) {
	// make sure to add .db to database name when using sqlite3
    switch config.Type {
    case "sqlite3":
		writeDB, err := openSQLiteConnection(config.Database, false)
        if err != nil {
            return nil, fmt.Errorf("sqlite write pool init failed: %w", err)
        }
        readDB, err := openSQLiteConnection(config.Database, true)
        if err != nil {
            return nil, fmt.Errorf("sqlite read pool init failed: %w", err)
        }

        if err := runMigrations(writeDB, "sqlite3"); err != nil {
            return nil, fmt.Errorf("sqlite migrations failed: %w", err)
        }

        return &DBPool{
            ReadDB:  readDB,
            WriteDB: writeDB,
            Type:    config.Type,
        }, nil

    case "postgres":
        pool, err := initPostgresPool(config)
        if err != nil {
            return nil, fmt.Errorf("postgres pool init failed: %w", err)
        }

        connStr := buildPostgresConnectionString(config)
        sqlDB, err := sql.Open("pgx", connStr)
        if err != nil {
            return nil, fmt.Errorf("postgres migration connection failed: %w", err)
        }

        if err := runMigrations(sqlDB, "postgres"); err != nil {
            sqlDB.Close()
            return nil, fmt.Errorf("postgres migrations failed: %w", err)
        }
        sqlDB.Close()

        return &DBPool{
            PgxPool: pool,
            Type:    config.Type,
        }, nil

    default:
        return nil, fmt.Errorf("unsupported database type: %s", config.Type)
    }
}

func buildPostgresConnectionString(config DatabaseConfig) string {
    sslMode := config.SSLMode
    if sslMode == "" {
        sslMode = "disable"
    }

    return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        config.Host, config.Port, config.User, config.Password, config.Database, sslMode)
}

func initPostgresPool(config DatabaseConfig) (*pgxpool.Pool, error) {
    connStr := buildPostgresConnectionString(config)

    poolConfig, err := pgxpool.ParseConfig(connStr)
    if err != nil {
        return nil, err
    }

    poolConfig.MaxConns = 25
    poolConfig.MinConns = 5
    poolConfig.MaxConnLifetime = time.Hour
    poolConfig.MaxConnIdleTime = 30 * time.Minute

    pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
    if err != nil {
        return nil, err
    }

    if err := pool.Ping(context.Background()); err != nil {
        return nil, fmt.Errorf("postgres pool ping failed: %w", err)
    }

    return pool, nil
}

func runMigrations(db *sql.DB, dbType string) error {
	var driver database.Driver
    var err error
    var migrationPath string

    switch dbType {
    case "sqlite3":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
        migrationPath = "file://migrations/sqlite"
    case "postgres":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
        migrationPath = "file://migrations/postgres"
    default:
        return fmt.Errorf("unsupported database type for migrations: %s", dbType)
    }

    if err != nil {
        return err
    }

    m, err := migrate.NewWithDatabaseInstance(migrationPath, dbType, driver)
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
    PgxPool *pgxpool.Pool
    Type    string
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

func openSQLiteConnection(database string, readonly bool) (*sql.DB, error) {
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
    switch pool.Type {
    case "postgres":
        return InsertUserPG(pool, ctx, email, username, password)
    case "sqlite3":
        return InsertUserSQLite(pool, ctx, email, username, password)
    default:
        return fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func GetAllUsers(pool *DBPool, ctx context.Context) ([]User, error) {
    switch pool.Type {
    case "postgres":
        return GetAllUsersPG(pool, ctx)
    case "sqlite3":
        return GetAllUsersSQLite(pool, ctx)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func GetUserByEmail(pool *DBPool, ctx context.Context, email string) (*User, error) {
    switch pool.Type {
    case "postgres":
        return GetUserByEmailPG(pool, ctx, email)
    case "sqlite3":
        return GetUserByEmailSQLite(pool, ctx, email)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func StoreRefreshToken(pool *DBPool, ctx context.Context, refreshToken auth.RefreshToken) error {
    switch pool.Type {
    case "postgres":
        return StoreRefreshTokenPG(pool, ctx, refreshToken)
    case "sqlite3":
        return StoreRefreshTokenSQLite(pool, ctx, refreshToken)
    default:
        return fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func GetValidRefreshTokenForUser(pool *DBPool, ctx context.Context, userID string) (*auth.RefreshToken, error) {
    switch pool.Type {
    case "postgres":
        return GetValidRefreshTokenForUserPG(pool, ctx, userID)
    case "sqlite3":
        return GetValidRefreshTokenForUserSQLite(pool, ctx, userID)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func RevokeRefreshTokensForUser(pool *DBPool, ctx context.Context, userID string) error {
    switch pool.Type {
    case "postgres":
        return RevokeRefreshTokensForUserPG(pool, ctx, userID)
    case "sqlite3":
        return RevokeRefreshTokensForUserSQLite(pool, ctx, userID)
    default:
        return fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}
