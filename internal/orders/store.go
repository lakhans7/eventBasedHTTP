// Package orders is a deliberately minimal local order log for a
// single-product store: an append-only JSON-lines file guarded by a mutex.
// A real database is unnecessary machinery for one SKU at low volume — see
// the "don't introduce unnecessary libraries/abstractions" principle. Every
// order is also attached to its Razorpay Order's `notes` field, so the
// Razorpay dashboard alone is enough to fulfill orders even if this file is
// never read.
package orders

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Order struct {
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	CustomerName      string    `json:"customer_name"`
	Phone             string    `json:"phone"`
	Email             string    `json:"email,omitempty"`
	AddressLine       string    `json:"address_line"`
	City              string    `json:"city"`
	State             string    `json:"state"`
	Pincode           string    `json:"pincode"`
	Size              string    `json:"size"`
	Quantity          int       `json:"quantity"`
	AmountCents       int64     `json:"amount_cents"`
	Currency          string    `json:"currency"`
	RazorpayOrderID   string    `json:"razorpay_order_id"`
	RazorpayPaymentID string    `json:"razorpay_payment_id,omitempty"`
	Status            string    `json:"status"` // "created" | "paid"
}

const (
	StatusCreated = "created"
	StatusPaid    = "paid"
)

type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Append(o Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendLocked(o)
}

func (s *Store) appendLocked(o Order) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// MarkPaid appends a new "paid" line for the given Razorpay order rather
// than rewriting history in place, so the file stays a full audit trail:
// the original "created" line followed by a "paid" line once payment is
// confirmed. Returns os.ErrNotExist if no order with that id was logged.
func (s *Store) MarkPaid(razorpayOrderID, razorpayPaymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readAllLocked()
	if err != nil {
		return err
	}
	var found *Order
	for i := range existing {
		if existing[i].RazorpayOrderID == razorpayOrderID {
			found = &existing[i]
		}
	}
	if found == nil {
		return os.ErrNotExist
	}
	if found.Status == StatusPaid {
		return nil // already recorded — keep MarkPaid idempotent for retried webhooks
	}

	updated := *found
	updated.Status = StatusPaid
	updated.RazorpayPaymentID = razorpayPaymentID
	updated.CreatedAt = time.Now()
	return s.appendLocked(updated)
}

func (s *Store) readAllLocked() ([]Order, error) {
	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Order
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var o Order
		if err := json.Unmarshal(scanner.Bytes(), &o); err != nil {
			continue // skip a malformed line rather than fail the whole read
		}
		out = append(out, o)
	}
	return out, scanner.Err()
}
