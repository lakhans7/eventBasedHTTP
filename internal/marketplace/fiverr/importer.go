package fiverr

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

// SkippedRow records a CSV row that could not be imported. Per docs/security.md,
// a malformed row must never abort the whole import — it's reported and the
// rest of the file still gets processed.
type SkippedRow struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Imported int          `json:"imported"`
	Skipped  []SkippedRow `json:"skipped"`
}

// header maps a CSV file's header row (case-insensitive, trimmed) to column indices.
type header map[string]int

func readHeader(r *csv.Reader) (header, error) {
	row, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV header: %w", err)
	}
	h := make(header, len(row))
	for i, col := range row {
		h[strings.ToLower(strings.TrimSpace(col))] = i
	}
	return h, nil
}

func (h header) get(row []string, name string) string {
	i, ok := h[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseCents(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(strings.TrimPrefix(s, "$"), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	return int64(f * 100), nil
}

func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid date %q", s)
}

// ImportGigs parses "title,description,category,status,base_price_cents,currency" rows.
func ImportGigs(ctx context.Context, st *store.Store, accountID string, r io.Reader) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	h, err := readHeader(cr)
	if err != nil {
		return nil, err
	}

	res := &ImportResult{}
	line := 1
	for {
		row, err := cr.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}

		title := h.get(row, "title")
		if title == "" {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: "missing required column: title"})
			continue
		}
		priceCents, err := parseCents(h.get(row, "base_price_cents"))
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}

		status := h.get(row, "status")
		if status == "" {
			status = domain.GigStatusUnknown
		}
		currency := h.get(row, "currency")

		_, err = st.UpsertGig(ctx, store.GigInput{
			FiverrAccountID: accountID,
			Title:           title,
			Description:     h.get(row, "description"),
			Category:        h.get(row, "category"),
			Status:          status,
			BasePriceCents:  priceCents,
			Currency:        currency,
			Source:          domain.GigSourceImport,
		})
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}
		res.Imported++
	}
	return res, nil
}

// ImportOrders parses "customer_username,customer_display_name,external_ref,amount_cents,currency,status,stage,due_at" rows.
func ImportOrders(ctx context.Context, st *store.Store, accountID string, r io.Reader) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	h, err := readHeader(cr)
	if err != nil {
		return nil, err
	}

	res := &ImportResult{}
	line := 1
	for {
		row, err := cr.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}

		customerUsername := h.get(row, "customer_username")
		if customerUsername == "" {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: "missing required column: customer_username"})
			continue
		}
		amountCents, err := parseCents(h.get(row, "amount_cents"))
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}
		dueAt, err := parseTime(h.get(row, "due_at"))
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}

		customer, err := st.GetOrCreateCustomer(ctx, accountID, customerUsername, h.get(row, "customer_display_name"))
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: "could not resolve customer: " + err.Error()})
			continue
		}

		status := h.get(row, "status")
		stage := h.get(row, "stage")
		var externalRef *string
		if v := h.get(row, "external_ref"); v != "" {
			externalRef = &v
		}

		_, err = st.CreateOrder(ctx, store.OrderInput{
			FiverrAccountID: accountID,
			CustomerID:      customer.ID,
			ExternalRef:     externalRef,
			AmountCents:     amountCents,
			Currency:        h.get(row, "currency"),
			Status:          status,
			Stage:           stage,
			DueAt:           dueAt,
			Source:          domain.OrderSourceImport,
		})
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}
		res.Imported++
	}
	return res, nil
}

// ImportReviews parses "rating,body,posted_at" rows.
func ImportReviews(ctx context.Context, st *store.Store, accountID string, r io.Reader) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	h, err := readHeader(cr)
	if err != nil {
		return nil, err
	}

	res := &ImportResult{}
	line := 1
	for {
		row, err := cr.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}

		ratingStr := h.get(row, "rating")
		rating, err := strconv.Atoi(ratingStr)
		if err != nil || rating < 1 || rating > 5 {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: fmt.Sprintf("invalid rating %q (must be 1-5)", ratingStr)})
			continue
		}
		postedAt, err := parseTime(h.get(row, "posted_at"))
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}
		posted := time.Now()
		if postedAt != nil {
			posted = *postedAt
		}

		_, err = st.CreateReview(ctx, store.ReviewInput{
			FiverrAccountID: accountID,
			Rating:          rating,
			Body:            h.get(row, "body"),
			PostedAt:        posted,
		})
		if err != nil {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: err.Error()})
			continue
		}
		res.Imported++
	}
	return res, nil
}
