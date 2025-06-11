package chat

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
)

func StressTestChat(baseURL string, numUsers, messagesPerUser int) {
    var wg sync.WaitGroup
    
    start := time.Now()
    
    for i := 0; i < numUsers; i++ {
        wg.Add(1)
        go func(userNum int) {
            defer wg.Done()

            for j := 0; j < messagesPerUser; j++ {
                msg := SendMessageRequest{
                    Content: fmt.Sprintf("Message %d from user %d", j, userNum),
                    RoomID:  "general",
                }
                
                jsonData, _ := json.Marshal(msg)
                resp, err := http.Post(baseURL+"/api/chat/send", "application/json", bytes.NewBuffer(jsonData))
                if err != nil {
                    fmt.Printf("Error: %v\n", err)
                    continue
                }
                resp.Body.Close()
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(start)
    totalMessages := numUsers * messagesPerUser
    
    fmt.Printf("Sent %d messages in %v (%.2f msg/sec)\n", 
        totalMessages, duration, float64(totalMessages)/duration.Seconds())
}

