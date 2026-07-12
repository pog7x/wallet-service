// Package api implements the HTTP transport layer for the wallet service.
//
// The package translates HTTP requests into Service calls and Service results
// into HTTP responses. It contains no business rules: all invariants are
// enforced by the Service, and the handlers only perform syntactic validation
// of incoming payloads and map domain errors to status codes.
//
// The package depends on the Service through a narrow interface declared here,
// so the Service package has no knowledge of HTTP.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/pog7x/wallet-service/internal/account"
	"github.com/pog7x/wallet-service/internal/money"
)

const (
	// maxTransferBodyBytes bounds the size of a single-transfer request body.
	// A transfer object is a handful of short fields, so a kilobyte is generous;
	// the bound exists so that a client cannot make the server buffer an
	// arbitrarily large payload before the first field is even parsed.
	maxTransferBodyBytes int64 = 1 << 10 // 1 KiB

	// maxBatchBodyBytes bounds the size of a batch request body. It is derived
	// from maxBatchEntries: an entry is well under 128 bytes, so this leaves
	// ample headroom while still rejecting payloads that could exhaust memory.
	maxBatchBodyBytes int64 = 128 << 10 // 128 KiB

	// maxBatchEntries bounds the number of transfers in one batch. Without an
	// upper bound the handler would allocate a slice proportional to whatever
	// the client sent, and the semaphore would only limit concurrency, not the
	// memory already committed to holding the requests.
	maxBatchEntries int = 1000
)

// Service is the subset of the wallet service required by the HTTP handlers.
//
// The interface is declared here, on the consumer side, so that the handlers
// depend only on the operations they actually call and the account package has
// no knowledge of HTTP. Tests substitute an implementation that returns
// arbitrary domain errors.
//
// Every method receives the request context and must abort when it is cancelled.
type Service interface {
	Transfer(ctx context.Context, fromID, toID string, amount money.Money) error
	TransferBatch(ctx context.Context, reqs []account.BatchRequest, concurrency int) []error
}

// Handler serves the wallet HTTP API.
//
// A Handler is safe for concurrent use: it holds no per-request state, and all
// mutable state lives in the underlying Service.
type Handler struct {
	svc        Service
	batchLimit int
}

// NewHandler returns a Handler that serves the API on top of svc.
//
// svc must not be nil. A nil svc panics here, at wiring time, rather than on the
// first request: a service that fails to start is a smaller problem than a
// service that starts and then fails on the first transfer.
func NewHandler(svc Service, batchLimit int) *Handler {
	if svc == nil {
		panic("api: NewHandler called with a nil service")
	}
	return &Handler{svc: svc, batchLimit: batchLimit}
}

// Routes registers the API endpoints on mux.
//
// Each route is bound to a single HTTP method. A request with a matching path
// but a different method is rejected with 405 Method Not Allowed by the mux
// itself, so handlers never inspect r.Method.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /transfers", h.TransferHandler)
	mux.HandleFunc("POST /transfers/batch", h.TransferBatchHandler)
}

// TransferHandler handles POST /transfers.
//
// The request body must be a JSON object described by TransferRequest. On
// success the response carries no body, because the operation produces no data
// the caller does not already have.
//
// The operation is not idempotent: repeating a successful request performs a
// second transfer. A caller that retries must first establish that the previous
// attempt did not succeed.
//
// Status codes:
//   - 204 No Content: the transfer was applied.
//   - 400 Bad Request: the body is malformed, contains an unknown field, an
//     account identifier or the currency is empty, or the amount is not positive.
//   - 404 Not Found: the source or the destination account does not exist.
//   - 409 Conflict: the source account has insufficient funds, or the amount
//     currency differs from the account currency.
//   - 413 Request Entity Too Large: the body exceeds the size limit (or, for the
//     batch endpoint, the batch contains too many entries).
//   - 500 Internal Server Error: any other failure of the Service.
//
// The request context is passed to the Service unchanged, so cancelling the
// request aborts the transfer. Whether the transfer had already been committed
// at the moment of cancellation is not defined by this contract, so a cancelled
// request must be treated as an unknown outcome.
func (h *Handler) TransferHandler(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := decodeStrict(w, r, maxTransferBodyBytes, &req); err != nil {
		writeDecodeError(w, err, "request body is not a valid transfer object")
		return
	}

	if req.From == "" || req.To == "" || req.Currency == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"from, to and currency must be non-empty and amount must be strictly positive")
		return
	}

	err := h.svc.Transfer(r.Context(), req.From, req.To, money.New(req.Amount, money.Currency(req.Currency)))
	if err != nil {
		m := responseMappingFor(err)
		writeError(w, m.Status, m.Code, m.Message)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TransferBatchHandler handles POST /transfers/batch.
//
// The transfers in the batch are executed concurrently and independently, with
// at most h.batchLimit of them in flight at a time. The handler does not stop
// at the first failure: every entry is attempted.
//
// The response body reports the outcome of every entry in request order, so the
// caller can distinguish the transfers that were applied from the ones that were
// not. Only the entries reported as failed may be safely retried; the operation
// is not idempotent, and retrying the whole batch would duplicate the transfers
// that had succeeded.
//
// Status codes:
//   - 200 OK: the batch was processed. Individual transfers may still have
//     failed; the caller must inspect the per-entry outcomes in the body.
//   - 400 Bad Request: the body is not valid JSON, the batch is empty, or an
//     entry is syntactically invalid.
//   - 413 Request Entity Too Large: the body exceeds the size limit (or, for the
//     batch endpoint, the batch contains too many entries).
//
// The request context is passed to the Service unchanged, so cancelling the
// request aborts the transfers that have not started or completed yet. Transfers
// already committed at that moment are not rolled back, and their outcomes are
// still reported.
func (h *Handler) TransferBatchHandler(w http.ResponseWriter, r *http.Request) {
	var req TransferBatchRequest
	if err := decodeStrict(w, r, maxBatchBodyBytes, &req); err != nil {
		writeDecodeError(w, err, "request body is not a valid batch object")
		return
	}

	if len(req.Batch) == 0 {
		writeError(w, http.StatusBadRequest, "empty_batch", "batch must contain at least one transfer")
		return
	}

	if len(req.Batch) > maxBatchEntries {
		writeError(w, http.StatusRequestEntityTooLarge, "batch_too_large",
			fmt.Sprintf("batch must not contain more than %d transfers", maxBatchEntries))
		return
	}

	accBatch := make([]account.BatchRequest, len(req.Batch))
	for i, b := range req.Batch {
		if b.From == "" || b.To == "" || b.Currency == "" || b.Amount <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_entry",
				fmt.Sprintf("entry %d: from, to and currency must be non-empty and amount must be strictly positive", i))
			return
		}
		accBatch[i] = account.BatchRequest{
			From:   b.From,
			To:     b.To,
			Amount: money.New(b.Amount, money.Currency(b.Currency)),
		}
	}

	errs := h.svc.TransferBatch(r.Context(), accBatch, h.batchLimit)

	resp := TransferBatchResponse{
		Results: make([]TransferResultItem, len(errs)),
	}
	for i, e := range errs {
		if e == nil {
			resp.Results[i] = TransferResultItem{Status: http.StatusOK}
			resp.Succeeded++
			continue
		}
		m := mappingFor(e)
		resp.Results[i] = TransferResultItem{
			Status: m.Status,
			Error:  &ErrorDetail{Code: m.Code, Message: m.Message},
		}
		resp.Failed++
	}

	writeJSON(w, http.StatusOK, resp)
}

// errorMapping describes how a single domain error is exposed over HTTP.
//
// Status is the HTTP status code, Code is a stable machine-readable identifier
// that clients may branch on, and Message is a human-readable explanation that
// may change without notice.
type errorMapping struct {
	Status  int
	Code    string
	Message string
}

// statusClientClosedRequest is the non-standard status code introduced by nginx
// for a request aborted by the client. It is used only inside per-entry batch
// results, because a client that aborted the request never receives a response.
const statusClientClosedRequest = 499

// errorMappings is the single source of truth for translating domain errors into
// HTTP responses. Every domain error exposed by the API must appear here exactly
// once; keeping status, code and message in one entry makes it impossible for
// them to drift apart when a new error is added.
//
// The table is ordered: entries are matched from top to bottom with errors.Is,
// so more specific errors must precede more general ones.
var errorMappings = []struct {
	err error
	errorMapping
}{
	{account.ErrNonPositiveAmount, errorMapping{http.StatusBadRequest, "non_positive_amount", "amount must be strictly positive"}},
	{account.ErrSameAccount, errorMapping{http.StatusBadRequest, "same_account", "source and destination accounts must differ"}},
	{account.ErrAccountNotFound, errorMapping{http.StatusNotFound, "account_not_found", "account does not exist"}},
	{account.ErrInsufficientFunds, errorMapping{http.StatusConflict, "insufficient_funds", "source account has insufficient funds"}},
	{account.ErrCurrencyMismatch, errorMapping{http.StatusConflict, "currency_mismatch", "amount currency differs from the account currency"}},
	{context.Canceled, errorMapping{statusClientClosedRequest, "canceled", "request was canceled"}},
	{context.DeadlineExceeded, errorMapping{http.StatusGatewayTimeout, "deadline_exceeded", "operation timed out"}},
}

// internalMapping is used for any error that is not present in errorMappings.
//
// An unrecognised failure cannot be attributed to the caller, so it is reported
// as a server error, and its text is never exposed.
var internalMapping = errorMapping{http.StatusInternalServerError, "internal", "internal error"}

// mappingFor returns the HTTP representation of err.
//
// Errors are matched with errors.Is, so wrapped errors are recognised. An error
// that matches no entry in errorMappings yields internalMapping, because an
// unrecognised failure cannot be attributed to the caller.
//
// err must not be nil. A nil error has no HTTP representation: success is not a
// mapping, and callers must handle the success path before consulting this
// function. Passing nil panics rather than returning a plausible-looking value,
// so that the mistake surfaces at the first test run instead of silently turning
// a successful operation into an error response.
func mappingFor(err error) errorMapping {
	if err == nil {
		panic("transport: mappingFor called with a nil error")
	}
	for _, m := range errorMappings {
		if errors.Is(err, m.err) {
			return m.errorMapping
		}
	}
	return internalMapping
}

// responseMappingFor returns the HTTP representation of err for a response
// whose status line the caller will see.
//
// context.Canceled is not exposed as a status: the client that cancelled the
// request is gone, and statusClientClosedRequest is not a real HTTP status
// code, so it must never appear in a status line. It is reported as 499 only
// inside per-entry batch results, where it is an ordinary number in a JSON body.
func responseMappingFor(err error) errorMapping {
	m := mappingFor(err)
	if m.Status == statusClientClosedRequest {
		m.Status = http.StatusInternalServerError
	}
	return m
}

// decodeStrict decodes the request body into dst.
//
// The body is limited to maxBytes: a larger body is rejected instead of being
// buffered, so a client cannot force the server to allocate memory proportional
// to what it chose to send. Unknown fields are rejected as well, so a misspelled
// field name is reported to the caller instead of being silently ignored.
//
// The returned error is *http.MaxBytesError when the limit was exceeded, which
// lets the caller answer 413 instead of 400.
func decodeStrict(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeDecodeError answers a failed decode, distinguishing an oversized body
// from a malformed one, because the two require different actions from the
// caller: the first must send less, the second must send valid JSON.
func writeDecodeError(w http.ResponseWriter, err error, invalidMessage string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large",
			fmt.Sprintf("request body must not exceed %d bytes", tooLarge.Limit))
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_body", invalidMessage)
}

// writeJSON writes v as a JSON body with the given status code.
//
// The status code is written before the body, because net/http fixes the status
// on the first write and any later WriteHeader call is ignored.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("transport: encode response: %v", err)
	}
}

// writeError writes an ErrorResponse with the given status code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}
