package appcontext

import (
    "context"
    "log"
    "net/http"
    "sync"
    "bytes"
    "gooner/db"
)

// AppContext holds request-scoped data and dependencies.
// Pool is optional and can be nil if not needed.
// Logger can be per-router or per-request.
type AppContext struct {
    context.Context
    Writer  http.ResponseWriter
    Request *http.Request
    Logger  *log.Logger
    Pool    *db.DBPool
}

// sync.Pool for AppContext reuse
var appContextPool = sync.Pool{
    New: func() any {
        return new(AppContext)
	},
}

// CleanPut resets AppContext fields and puts it back to the pool
func CleanPut(ctx *AppContext) {
    ctx.Context = nil
    ctx.Writer = nil
    ctx.Request = nil
    ctx.Logger = nil
    ctx.Pool = nil
    appContextPool.Put(ctx)
}

// sync.Pool for bytes.Buffer reuse (for fmt buffer usage)
var fmtBufferPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

// CleanPutFmtBuffer resets the buffer and puts it back to the pool
func CleanPutFmtBuffer(buf *bytes.Buffer) {
    buf.Reset()
    fmtBufferPool.Put(buf)
}

// GetAppContext retrieves an AppContext from the pool
func GetAppContext() *AppContext {
    return appContextPool.Get().(*AppContext)
}

// GetFmtBuffer retrieves a bytes.Buffer from the pool
func GetFmtBuffer() *bytes.Buffer {
    return fmtBufferPool.Get().(*bytes.Buffer)
}

