package webhooks

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
    "net/http"
    "gooner/appcontext"
)

type WebhookHandler struct {
    Secret string
}

func NewWebhookHandler(secret string) *WebhookHandler {
    return &WebhookHandler{Secret: secret}
}

func (wh *WebhookHandler) VerifySignature(payload []byte, signature string) bool {
    if wh.Secret == "" {
        return true // Skip verification if no secret configured
    }
    
    mac := hmac.New(sha256.New, []byte(wh.Secret))
    mac.Write(payload)
    expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func (wh *WebhookHandler) StripeWebhook(ctx *appcontext.AppContext) {
    body, err := io.ReadAll(ctx.Request.Body)
    if err != nil {
        ctx.Writer.WriteHeader(http.StatusBadRequest)
        return
    }
    
    signature := ctx.Request.Header.Get("Stripe-Signature")
    if !wh.VerifySignature(body, signature) {
        ctx.Writer.WriteHeader(http.StatusUnauthorized)
        return
    }
    
    // Process Stripe webhook
    ctx.Logger.Printf("Received Stripe webhook: %s", string(body))
    
    // TODO: Parse webhook event and handle accordingly
    // This is where you'll integrate with your job system later
    
    ctx.Writer.WriteHeader(http.StatusOK)
}

func (wh *WebhookHandler) GenericWebhook(ctx *appcontext.AppContext) {
    body, err := io.ReadAll(ctx.Request.Body)
    if err != nil {
        ctx.Writer.WriteHeader(http.StatusBadRequest)
        return
    }
    
    signature := ctx.Request.Header.Get("X-Webhook-Signature")
    if !wh.VerifySignature(body, signature) {
        ctx.Writer.WriteHeader(http.StatusUnauthorized)
        return
    }
    
    ctx.Logger.Printf("Received webhook: %s", string(body))
    ctx.Writer.WriteHeader(http.StatusOK)
}
