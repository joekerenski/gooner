package chat

import (
    "time"
)

type Message struct {
    ID        int       `json:"id"`
    UserID    string    `json:"user_id"`
    Username  string    `json:"username"`
    Content   string    `json:"content"`
    RoomID    string    `json:"room_id"`
    CreatedAt time.Time `json:"created_at"`
}

type Room struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}

type SendMessageRequest struct {
    Content string `json:"content"`
    RoomID  string `json:"room_id"`
}

type GetMessagesResponse struct {
    Messages []Message `json:"messages"`
    Total    int       `json:"total"`
}
