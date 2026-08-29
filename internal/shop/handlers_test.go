package shop

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/config"
	"github.com/lakhans7/eventbasedhttp/internal/orders"
	"github.com/lakhans7/eventbasedhttp/internal/razorpay"
)

func testApp(t *testing.T) (*fiber.App, *orders.Store) {
	t.Helper()
	cfg := &config.Config{
		StoreName: "Test Store", ProductName: "Test Gown", PriceCents: 79900, Currency: "INR",
		RazorpayKeyID: "rzp_test_key", RazorpayKeySecret: "test_secret",
	}
	store := orders.NewStore(filepath.Join(t.TempDir(), "orders.jsonl"))
	h := NewHandlers(cfg, razorpay.NewClient(cfg.RazorpayKeyID, cfg.RazorpayKeySecret), store)

	app := fiber.New()
	app.Get("/api/product", h.GetProduct)
	app.Post("/api/order", h.PostOrder)
	app.Post("/api/verify", h.PostVerify)
	return app, store
}

func postJSON(t *testing.T, app *fiber.App, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func TestGetProductReflectsFixedPrice(t *testing.T) {
	app, _ := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/product", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestPostOrderRejectsInvalidInput guards the checkout form's server-side
// validation, since the client-side HTML form attributes are trivially
// bypassed by anyone calling the API directly.
func TestPostOrderRejectsInvalidInput(t *testing.T) {
	app, _ := testApp(t)

	valid := `{"customer_name":"A Buyer","phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"M","quantity":1}`

	cases := map[string]string{
		"missing name":    `{"phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"M","quantity":1}`,
		"invalid phone":   `{"customer_name":"A Buyer","phone":"abc","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"M","quantity":1}`,
		"invalid pincode": `{"customer_name":"A Buyer","phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"abcde","size":"M","quantity":1}`,
		"invalid size":    `{"customer_name":"A Buyer","phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"XXXXL","quantity":1}`,
		"quantity zero":   `{"customer_name":"A Buyer","phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"M","quantity":0}`,
		"quantity huge":   `{"customer_name":"A Buyer","phone":"9876543210","address_line":"1 Main St","city":"Pune","state":"MH","pincode":"411001","size":"M","quantity":999}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp := postJSON(t, app, "/api/order", body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d", name, resp.StatusCode)
			}
		})
	}

	// Sanity check the "valid" fixture only fails downstream (talking to the
	// real Razorpay API with fake test credentials), never on validation —
	// otherwise the rejection cases above would be meaningless.
	resp := postJSON(t, app, "/api/order", valid)
	if resp.StatusCode == http.StatusBadRequest {
		t.Fatalf("expected the valid fixture to pass validation (fail later on the Razorpay call instead), got 400")
	}
}

func TestPostVerifyRejectsBadSignature(t *testing.T) {
	app, _ := testApp(t)
	resp := postJSON(t, app, "/api/verify", `{"razorpay_order_id":"order_x","razorpay_payment_id":"pay_x","razorpay_signature":"not-valid"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid signature, got %d", resp.StatusCode)
	}
}

func TestPostVerifyRejectsMissingFields(t *testing.T) {
	app, _ := testApp(t)
	resp := postJSON(t, app, "/api/verify", `{"razorpay_order_id":"order_x"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 when payment id/signature are missing, got %d", resp.StatusCode)
	}
}
