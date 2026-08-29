package orders

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(filepath.Join(dir, "orders.jsonl"))
}

func TestAppendAndMarkPaid(t *testing.T) {
	st := newTestStore(t)

	err := st.Append(Order{
		ID:              "local-1",
		CreatedAt:       time.Now(),
		CustomerName:    "Test Buyer",
		Phone:           "9876543210",
		AddressLine:     "1 Test St",
		City:            "Testville",
		State:           "TS",
		Pincode:         "123456",
		Size:            "M",
		Quantity:        2,
		AmountCents:     159800,
		Currency:        "INR",
		RazorpayOrderID: "order_test1",
		Status:          StatusCreated,
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	if err := st.MarkPaid("order_test1", "pay_test1"); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}

	all, err := st.readAllLocked()
	if err != nil {
		t.Fatalf("readAllLocked: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 lines (created + paid), got %d", len(all))
	}
	last := all[len(all)-1]
	if last.Status != StatusPaid || last.RazorpayPaymentID != "pay_test1" {
		t.Fatalf("expected the latest record to be marked paid with the payment id, got %+v", last)
	}
	// The original order details must survive onto the "paid" record.
	if last.CustomerName != "Test Buyer" || last.Size != "M" || last.Quantity != 2 {
		t.Fatalf("MarkPaid must preserve the original order's details, got %+v", last)
	}
}

func TestMarkPaidIsIdempotent(t *testing.T) {
	st := newTestStore(t)
	_ = st.Append(Order{ID: "local-2", RazorpayOrderID: "order_test2", Status: StatusCreated})

	if err := st.MarkPaid("order_test2", "pay_A"); err != nil {
		t.Fatalf("first MarkPaid: %v", err)
	}
	// A retried webhook delivery must not add a duplicate "paid" line.
	if err := st.MarkPaid("order_test2", "pay_A"); err != nil {
		t.Fatalf("second MarkPaid: %v", err)
	}

	all, _ := st.readAllLocked()
	paidCount := 0
	for _, o := range all {
		if o.Status == StatusPaid {
			paidCount++
		}
	}
	if paidCount != 1 {
		t.Fatalf("expected exactly 1 paid record after a repeated MarkPaid, got %d", paidCount)
	}
}

func TestMarkPaidUnknownOrderReturnsNotExist(t *testing.T) {
	st := newTestStore(t)
	if err := st.MarkPaid("order_never_created", "pay_x"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist for an unknown order id, got %v", err)
	}
}
