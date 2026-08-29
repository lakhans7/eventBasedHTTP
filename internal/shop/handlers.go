// Package shop wires the storefront's HTTP handlers: creating a Razorpay
// order (server-decided amount, so the client can never pay less than the
// listed price), verifying the payment signature Razorpay's Checkout
// returns, and an optional webhook for robustness if the browser never
// calls back (closed tab, network drop) after a successful payment.
package shop

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/lakhans7/eventbasedhttp/internal/config"
	"github.com/lakhans7/eventbasedhttp/internal/orders"
	"github.com/lakhans7/eventbasedhttp/internal/razorpay"
)

var allowedSizes = map[string]bool{
	"S": true, "M": true, "L": true, "XL": true, "XXL": true, "XXXL": true,
}

var phoneRE = regexp.MustCompile(`^[0-9+][0-9 -]{7,14}$`)
var pincodeRE = regexp.MustCompile(`^[0-9]{4,10}$`)

type Handlers struct {
	cfg   *config.Config
	rzp   *razorpay.Client
	store *orders.Store
}

func NewHandlers(cfg *config.Config, rzp *razorpay.Client, store *orders.Store) *Handlers {
	return &Handlers{cfg: cfg, rzp: rzp, store: store}
}

func errJSON(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// GET /api/product — the frontend fetches price/config here rather than
// hardcoding it in HTML, so the price shown always matches what the server
// will actually charge.
func (h *Handlers) GetProduct(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"store_name":   h.cfg.StoreName,
		"product_name": h.cfg.ProductName,
		"price_cents":  h.cfg.PriceCents,
		"currency":     h.cfg.Currency,
		"sizes":        []string{"S", "M", "L", "XL", "XXL", "XXXL"},
		"razorpay_key": h.cfg.RazorpayKeyID,
	})
}

type createOrderRequest struct {
	CustomerName string `json:"customer_name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	AddressLine  string `json:"address_line"`
	City         string `json:"city"`
	State        string `json:"state"`
	Pincode      string `json:"pincode"`
	Size         string `json:"size"`
	Quantity     int    `json:"quantity"`
}

// PostOrder handles POST /api/order. The amount charged is always
// h.cfg.PriceCents * quantity — never a client-supplied value — so a
// tampered request can change the shipping address, not the price.
func (h *Handlers) PostOrder(c *fiber.Ctx) error {
	var req createOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "Could not parse request body.")
	}

	req.CustomerName = strings.TrimSpace(req.CustomerName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.AddressLine = strings.TrimSpace(req.AddressLine)
	req.City = strings.TrimSpace(req.City)
	req.State = strings.TrimSpace(req.State)
	req.Pincode = strings.TrimSpace(req.Pincode)
	req.Size = strings.ToUpper(strings.TrimSpace(req.Size))

	if req.CustomerName == "" || req.AddressLine == "" || req.City == "" || req.State == "" {
		return errJSON(c, fiber.StatusBadRequest, "Name and full address are required.")
	}
	if !phoneRE.MatchString(req.Phone) {
		return errJSON(c, fiber.StatusBadRequest, "A valid phone number is required.")
	}
	if !pincodeRE.MatchString(req.Pincode) {
		return errJSON(c, fiber.StatusBadRequest, "A valid pincode is required.")
	}
	if !allowedSizes[req.Size] {
		return errJSON(c, fiber.StatusBadRequest, "Select a valid size.")
	}
	if req.Quantity < 1 || req.Quantity > 10 {
		return errJSON(c, fiber.StatusBadRequest, "Quantity must be between 1 and 10.")
	}

	amountCents := h.cfg.PriceCents * int64(req.Quantity)
	orderID := uuid.New().String()

	rzpOrder, err := h.rzp.CreateOrder(razorpay.CreateOrderRequest{
		AmountCents: amountCents,
		Currency:    h.cfg.Currency,
		Receipt:     orderID,
		Notes: map[string]any{
			"customer_name": req.CustomerName,
			"phone":         req.Phone,
			"address":       req.AddressLine + ", " + req.City + ", " + req.State + " " + req.Pincode,
			"size":          req.Size,
			"quantity":      req.Quantity,
		},
	})
	if err != nil {
		return errJSON(c, fiber.StatusBadGateway, "Could not start payment. Please try again.")
	}

	if err := h.store.Append(orders.Order{
		ID:              orderID,
		CreatedAt:       time.Now(),
		CustomerName:    req.CustomerName,
		Phone:           req.Phone,
		Email:           req.Email,
		AddressLine:     req.AddressLine,
		City:            req.City,
		State:           req.State,
		Pincode:         req.Pincode,
		Size:            req.Size,
		Quantity:        req.Quantity,
		AmountCents:     amountCents,
		Currency:        h.cfg.Currency,
		RazorpayOrderID: rzpOrder.ID,
		Status:          orders.StatusCreated,
	}); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "Could not record order.")
	}

	return c.JSON(fiber.Map{
		"razorpay_order_id": rzpOrder.ID,
		"amount_cents":      amountCents,
		"currency":          h.cfg.Currency,
		"razorpay_key":      h.cfg.RazorpayKeyID,
	})
}

type verifyRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

// PostVerify handles POST /api/verify, called by the frontend's Razorpay
// Checkout success handler. See internal/razorpay.VerifyPaymentSignature —
// this is the step that actually confirms the payment is genuine; the
// checkout "success" callback alone is not trustworthy on its own.
func (h *Handlers) PostVerify(c *fiber.Ctx) error {
	var req verifyRequest
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "Could not parse request body.")
	}
	if req.RazorpayOrderID == "" || req.RazorpayPaymentID == "" || req.RazorpaySignature == "" {
		return errJSON(c, fiber.StatusBadRequest, "Missing payment confirmation fields.")
	}

	if !h.rzp.VerifyPaymentSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		return errJSON(c, fiber.StatusBadRequest, "Payment signature verification failed.")
	}

	if err := h.store.MarkPaid(req.RazorpayOrderID, req.RazorpayPaymentID); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "Payment verified but could not update order record.")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// PostWebhook handles POST /api/webhook/razorpay. Configure this URL (plus
// the same secret as RAZORPAY_WEBHOOK_SECRET) in the Razorpay dashboard for
// the "payment.captured" event — a safety net in case the browser never
// calls /api/verify (closed tab, dropped connection) after a real payment.
func (h *Handlers) PostWebhook(c *fiber.Ctx) error {
	if h.cfg.RazorpayWebhookSecret == "" {
		return errJSON(c, fiber.StatusNotFound, "Webhook not configured.")
	}

	raw := c.Body()
	signature := c.Get("X-Razorpay-Signature")
	if !razorpay.VerifyWebhookSignature(h.cfg.RazorpayWebhookSecret, raw, signature) {
		return errJSON(c, fiber.StatusBadRequest, "Invalid webhook signature.")
	}

	var event struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string `json:"id"`
					OrderID string `json:"order_id"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "Malformed webhook payload.")
	}

	if event.Event == "payment.captured" {
		_ = h.store.MarkPaid(event.Payload.Payment.Entity.OrderID, event.Payload.Payment.Entity.ID)
	}

	return c.SendStatus(fiber.StatusOK)
}
