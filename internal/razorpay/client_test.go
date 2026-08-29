package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// hmacHex independently reimplements Razorpay's documented signature formula
// (HMAC-SHA256, hex-encoded) so these tests exercise the production code
// against a computation that doesn't share any code with it.
func hmacHex(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyPaymentSignatureAcceptsCorrectSignature(t *testing.T) {
	c := NewClient("key_id", "test_secret")
	orderID, paymentID := "order_ABC123", "pay_XYZ789"
	validSignature := hmacHex("test_secret", orderID+"|"+paymentID)

	if !c.VerifyPaymentSignature(orderID, paymentID, validSignature) {
		t.Fatal("expected a correctly computed signature to verify")
	}
}

func TestVerifyPaymentSignatureRejectsTamperedValues(t *testing.T) {
	c := NewClient("key_id", "test_secret")
	validSignature := hmacHex("test_secret", "order_ABC123|pay_XYZ789")

	cases := []struct {
		name      string
		orderID   string
		paymentID string
		signature string
	}{
		{"wrong order id", "order_OTHER", "pay_XYZ789", validSignature},
		{"wrong payment id", "order_ABC123", "pay_OTHER", validSignature},
		{"garbage signature", "order_ABC123", "pay_XYZ789", "not-a-real-signature"},
		{"empty signature", "order_ABC123", "pay_XYZ789", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if c.VerifyPaymentSignature(tc.orderID, tc.paymentID, tc.signature) {
				t.Fatal("expected verification to fail for a tampered value")
			}
		})
	}
}

func TestVerifyPaymentSignatureRejectsWrongSecret(t *testing.T) {
	signedWithDifferentSecret := hmacHex("a-different-secret", "order_ABC123|pay_XYZ789")
	c := NewClient("key_id", "test_secret")
	if c.VerifyPaymentSignature("order_ABC123", "pay_XYZ789", signedWithDifferentSecret) {
		t.Fatal("a signature computed with a different key_secret must never verify")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"event":"payment.captured","payload":{}}`)
	valid := hmacHex("webhook_secret", string(body))

	if !VerifyWebhookSignature("webhook_secret", body, valid) {
		t.Fatal("expected the correctly computed webhook signature to verify")
	}
	if VerifyWebhookSignature("webhook_secret", []byte(`{"tampered":true}`), valid) {
		t.Fatal("a signature computed over a different body must never verify")
	}
	if VerifyWebhookSignature("", body, valid) {
		t.Fatal("an empty webhook secret (not configured) must never verify")
	}
}
