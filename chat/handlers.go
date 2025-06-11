package chat

import (
    "encoding/json"
    "net/http"
    "strconv"
	"sync"
	"time"
	"fmt"
    
    "gooner/appcontext"
)

func SendMessageHandler(ctx *appcontext.AppContext) {
    userID, ok := ctx.Context.Value("userID").(string)
    if !ok {
        http.Error(ctx.Writer, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req SendMessageRequest
    if err := json.NewDecoder(ctx.Request.Body).Decode(&req); err != nil {
        http.Error(ctx.Writer, "Invalid JSON", http.StatusBadRequest)
        return
    }

    if req.Content == "" || req.RoomID == "" {
        http.Error(ctx.Writer, "Content and room_id are required", http.StatusBadRequest)
        return
    }

    message, err := StoreMessage(ctx.Pool, ctx.Context, userID, req.RoomID, req.Content)
    if err != nil {
        ctx.Logger.Printf("Failed to store message: %v", err)
        http.Error(ctx.Writer, "Failed to send message", http.StatusInternalServerError)
        return
    }

    ctx.Writer.Header().Set("Content-Type", "application/json")
    json.NewEncoder(ctx.Writer).Encode(message)
}

func GetMessagesHandler(ctx *appcontext.AppContext) {
    roomID := ctx.Request.URL.Query().Get("room_id")
    if roomID == "" {
        roomID = "general" // Default room
    }

    limitStr := ctx.Request.URL.Query().Get("limit")
    limit := 50 // Default
    if limitStr != "" {
        if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
            limit = l
        }
    }

    offsetStr := ctx.Request.URL.Query().Get("offset")
    offset := 0
    if offsetStr != "" {
        if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
            offset = o
        }
    }

    messages, err := GetMessages(ctx.Pool, ctx.Context, roomID, limit, offset)
    if err != nil {
        ctx.Logger.Printf("Failed to get messages: %v", err)
        http.Error(ctx.Writer, "Failed to get messages", http.StatusInternalServerError)
        return
    }

    response := GetMessagesResponse{
        Messages: messages,
        Total:    len(messages),
    }

    ctx.Writer.Header().Set("Content-Type", "application/json")
    json.NewEncoder(ctx.Writer).Encode(response)
}

func StressTestHandler(ctx *appcontext.AppContext) {
    userID, ok := ctx.Context.Value("userID").(string)
    if !ok {
        http.Error(ctx.Writer, "Unauthorized", http.StatusUnauthorized)
        return
    }

    numUsers := 1000
    messagesPerUser := 25

    var wg sync.WaitGroup
    var mu sync.Mutex
    successCount := 0
    failCount := 0

    start := time.Now()

    for i := 0; i < numUsers; i++ {
        wg.Add(1)
        go func(userNum int) {
            defer wg.Done()
            for j := 0; j < messagesPerUser; j++ {
                content := fmt.Sprintf("Stress test message %d from user %d. We are transmitting a lot of data here. I apparently have a lot to say and this is how I say it. Lucy is a good cat.", j, userNum)

                _, err := StoreMessage(ctx.Pool, ctx.Context, userID, "general", content)

                mu.Lock()
                if err != nil {
                    failCount++
                } else {
                    successCount++
                }
                mu.Unlock()
            }
        }(i)
    }

    wg.Wait()
    duration := time.Since(start)

    result := map[string]interface{}{
        "total_messages":     numUsers * messagesPerUser,
        "duration":          duration.String(),
        "success_count":     successCount,
        "fail_count":        failCount,
        "messages_per_sec":  float64(successCount) / duration.Seconds(),
    }

    ctx.Writer.Header().Set("Content-Type", "application/json")
    json.NewEncoder(ctx.Writer).Encode(result)
}
