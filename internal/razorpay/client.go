// Package razorpay is a minimal client for the parts of Razorpay's official
// API this shop needs: creating an Order server-side (so the amount can
// never be tampered with by the client) and verifying the payment/webhook
// signatures Razorpay documents. No SDK — just their documented REST API
// and HMAC-SHA256 signature scheme, same approach as internal/ai's direct
// Anthropic HTTP client in the sibling Fiverr-platform branch.
package razorpay

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const ordersEndpoint = "https://api.razorpay.com/v1/orders"

type Client struct {
	keyID     string
	keySecret string
	http      *http.Client
}

func NewClient(keyID, keySecret string) *Client {
	return &Client{keyID: keyID, keySecret: keySecret, http: &http.Client{Timeout: 20 * time.Second}}
}

type CreateOrderRequest struct {
	AmountCents int64          // in the currency's smallest unit (paise for INR)
	Currency    string         // e.g. "INR"
	Receipt     string         // your own order/receipt reference, shown in the Razorpay dashboard
	Notes       map[string]any // shown in the Razorpay dashboard — used here to carry buyer/shipping details
}

type Order struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

func (c *Client) CreateOrder(req CreateOrderRequest) (*Order, error) {
	if c.keyID == "" || c.keySecret == "" {
		return nil, fmt.Errorf("razorpay: RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET are not configured")
	}

	body, err := json.Marshal(map[string]any{
		"amount":   req.AmountCents,
		"currency": req.Currency,
		"receipt":  req.Receipt,
		"notes":    req.Notes,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, ordersEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(c.keyID, c.keySecret)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("razorpay: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("razorpay: create order failed with status %d: %s", resp.StatusCode, string(raw))
	}

	var order Order
	if err := json.Unmarshal(raw, &order); err != nil {
		return nil, fmt.Errorf("razorpay: malformed order response: %w", err)
	}
	return &order, nil
}

// VerifyPaymentSignature implements Razorpay's documented client-side
// checkout verification: expected = HMAC_SHA256(order_id + "|" + payment_id, key_secret).
// See https://razorpay.com/docs/payments/payment-gateway/web-integration/standard/build-integration/#step-4-verify-payment-signature
func (c *Client) VerifyPaymentSignature(orderID, paymentID, signature string) bool {
	return verifyHMAC(c.keySecret, orderID+"|"+paymentID, signature)
}

// VerifyWebhookSignature implements Razorpay's documented webhook
// verification: expected = HMAC_SHA256(raw_request_body, webhook_secret).
// See https://razorpay.com/docs/webhooks/validate-test/
func VerifyWebhookSignature(webhookSecret string, rawBody []byte, signature string) bool {
	return verifyHMAC(webhookSecret, string(rawBody), signature)
}

func verifyHMAC(secret, payload, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
