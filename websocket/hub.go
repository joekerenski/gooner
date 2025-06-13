package websocket

import (
    "encoding/json"
    "log"
    "net/http"
    "github.com/gorilla/websocket"
    "gooner/appcontext"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        return true // Configure properly for production
    },
}

type Hub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

type Client struct {
    hub  *Hub
    conn *websocket.Conn
    send chan []byte
}

type JobNotification struct {
    JobID  int    `json:"job_id"`
    Status string `json:"status"`
    Type   string `json:"type"`
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[*Client]bool),
        broadcast:  make(chan []byte),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
            log.Printf("Client connected. Total: %d", len(h.clients))

        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
                log.Printf("Client disconnected. Total: %d", len(h.clients))
            }

        case message := <-h.broadcast:
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
        }
    }
}

func (h *Hub) BroadcastJobCompletion(jobID int, status, jobType string) {
    notification := JobNotification{
        JobID:  jobID,
        Status: status,
        Type:   jobType,
    }

    data, _ := json.Marshal(notification)
    h.broadcast <- data
}

func WebSocketHandler(hub *Hub) func(*appcontext.AppContext) {
    return func(ctx *appcontext.AppContext) {
        conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
        if err != nil {
            ctx.Logger.Printf("WebSocket upgrade failed: %v", err)
            return
        }

        client := &Client{
            hub:  hub,
            conn: conn,
            send: make(chan []byte, 256),
        }

        client.hub.register <- client

        go client.writePump()
        go client.readPump()
    }
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    for {
        _, _, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("WebSocket error: %v", err)
            }
            break
        }
        // Handle incoming messages if needed
    }
}

func (c *Client) writePump() {
    defer c.conn.Close()

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }
        }
    }
}
