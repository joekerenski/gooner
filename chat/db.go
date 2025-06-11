package chat

import (
    "context"
    "fmt"
    "time"

    "gooner/db"
)

func StoreMessage(pool *db.DBPool, ctx context.Context, userID, roomID, content string) (*Message, error) {
    writeTx, err := pool.GetWriteTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer writeTx.Rollback()

    query := `INSERT INTO chat_messages (user_id, room_id, content, created_at) 
              VALUES (?, ?, ?, ?)`
    
    now := time.Now()
    result, err := writeTx.ExecContext(ctx, query, userID, roomID, content, now)
    if err != nil {
        return nil, fmt.Errorf("failed to store message: %w", err)
    }

    messageID, err := result.LastInsertId()
    if err != nil {
        return nil, fmt.Errorf("failed to get message ID: %w", err)
    }

    if err = writeTx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    // Get username for response
    username, err := getUsernameByID(pool, ctx, userID)
    if err != nil {
        username = "Unknown"
    }

    return &Message{
        ID:        int(messageID),
        UserID:    userID,
        Username:  username,
        Content:   content,
        RoomID:    roomID,
        CreatedAt: now,
    }, nil
}

func GetMessages(pool *db.DBPool, ctx context.Context, roomID string, limit, offset int) ([]Message, error) {
    readTx, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin read transaction: %w", err)
    }
    defer readTx.Rollback()

    query := `SELECT m.id, m.user_id, u.username, m.content, m.room_id, m.created_at
              FROM chat_messages m
              JOIN users u ON m.user_id = u.user_id
              WHERE m.room_id = ?
              ORDER BY m.created_at DESC
              LIMIT ? OFFSET ?`

    rows, err := readTx.QueryContext(ctx, query, roomID, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("failed to query messages: %w", err)
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var msg Message
        err := rows.Scan(
            &msg.ID,
            &msg.UserID,
            &msg.Username,
            &msg.Content,
            &msg.RoomID,
            &msg.CreatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan message: %w", err)
        }
        messages = append(messages, msg)
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating messages: %w", err)
    }

    return messages, readTx.Commit()
}

func getUsernameByID(pool *db.DBPool, ctx context.Context, userID string) (string, error) {
    readTx, err := pool.GetReadTx(ctx)
    if err != nil {
        return "", err
    }
    defer readTx.Rollback()

    var username string
    query := `SELECT username FROM users WHERE user_id = ?`
    err = readTx.QueryRowContext(ctx, query, userID).Scan(&username)
    if err != nil {
        return "", err
    }

    readTx.Commit()
    return username, nil
}
