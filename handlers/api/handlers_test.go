package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/pog7x/wallet-service/internal/account"
	"github.com/pog7x/wallet-service/internal/money"
)

type contextKey string

const (
	ctxKey         contextKey = "test_context_key"
	ctxValue       string     = "test_context_value"
	testBatchLimit int        = 5
)

// fakeService is a stub implementation of the Service interface used by the
// handler tests. It records the arguments of the last call and returns a
// prepared result, so that every branch of the handler can be exercised without
// driving a real Service into the corresponding state.
type fakeService struct {
	called     int
	gotCtx     context.Context
	gotBatch   []account.BatchRequest
	gotLimit   int
	returnErrs []error

	transferCalled int
	transferCtx    context.Context
	transferFrom   string
	transferTo     string
	transferAmount money.Money
	transferErr    error
}

func (f *fakeService) TransferBatch(ctx context.Context, batch []account.BatchRequest, limit int) []error {
	f.called++
	f.gotCtx = ctx
	f.gotBatch = batch
	f.gotLimit = limit
	if f.returnErrs != nil {
		return f.returnErrs
	}
	return make([]error, len(batch))
}

func (f *fakeService) Transfer(ctx context.Context, from, to string, amount money.Money) error {
	f.transferCalled++
	f.transferCtx = ctx
	f.transferFrom = from
	f.transferTo = to
	f.transferAmount = amount
	return f.transferErr
}

// doRequest executes the handler against a fake request carrying body and
// returns the recorder. No real server or network is involved, because a handler
// is an ordinary function of two arguments.
func doTransferBatchRequest(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/transfers/batch", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.TransferBatchHandler(rec, req)
	return rec
}

func doTransferRequest(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/transfers", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.TransferHandler(rec, req)
	return rec
}

func decodeBatchResponse(t *testing.T, rec *httptest.ResponseRecorder) TransferBatchResponse {
	t.Helper()
	var resp TransferBatchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return resp
}

func TestNewHandler_NilServicePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler(nil) did not panic: a missing dependency must fail at wiring time")
		}
	}()

	NewHandler(nil, testBatchLimit)
}

func TestTransferBatchHandler_Success(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferBatchRequest(t, h, `{"batch":[
		{"from":"a","to":"b","amount":100,"currency":"USD"},
		{"from":"c","to":"d","amount":250,"currency":"EUR"}
	]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d and no error, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	resp := decodeBatchResponse(t, rec)
	if resp.Succeeded != 2 || resp.Failed != 0 {
		t.Errorf("succeeded/failed = %d/%d, want 2/0", resp.Succeeded, resp.Failed)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(resp.Results))
	}
	for i, r := range resp.Results {
		if r.Status != http.StatusOK || r.Error != nil {
			t.Errorf("results[%d] = %+v, want index %d, status 200, no error", i, r, i)
		}
	}
}

func TestTransferBatchHandler_RequestIsMappedToDomain(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferBatchRequest(t, h, `{"batch":[
		{"from":"alice","to":"bob","amount":100,"currency":"USD"},
		{"from":"carol","to":"dave","amount":250,"currency":"EUR"}
	]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	want := []account.BatchRequest{
		{From: "alice", To: "bob", Amount: money.New(100, money.Currency("USD"))},
		{From: "carol", To: "dave", Amount: money.New(250, money.Currency("EUR"))},
	}
	if !reflect.DeepEqual(svc.gotBatch, want) {
		t.Errorf("batch passed to service = %+v, want %+v", svc.gotBatch, want)
	}
	if svc.gotLimit != testBatchLimit {
		t.Errorf("limit passed to service = %d, want %d", svc.gotLimit, testBatchLimit)
	}
}

func TestTransferBatchHandler_BadRequest(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"malformed json", `{"batch":[`, "invalid_body"},
		{"not an object", `"hello"`, "invalid_body"},
		{"empty body", ``, "invalid_body"},
		{"missing batch", `{}`, "empty_batch"},
		{"empty batch", `{"batch":[]}`, "empty_batch"},
		{"zero amount", `{"batch":[{"from":"a","to":"b","amount":0,"currency":"USD"}]}`, "invalid_entry"},
		{"negative amount", `{"batch":[{"from":"a","to":"b","amount":-5,"currency":"USD"}]}`, "invalid_entry"},
		{"empty from", `{"batch":[{"from":"","to":"b","amount":10,"currency":"USD"}]}`, "invalid_entry"},
		{"empty to", `{"batch":[{"from":"a","to":"","amount":10,"currency":"USD"}]}`, "invalid_entry"},
		{"empty currency", `{"batch":[{"from":"a","to":"b","amount":10,"currency":""}]}`, "invalid_entry"},
		{"unknown field in body", `{"batch":[],"limit":5}`, "invalid_body"},
		{"unknown field in entry", `{"batch":[{"from":"a","to":"b","amount":10,"currency":"USD","fee":1}]}`, "invalid_body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			h := NewHandler(svc, testBatchLimit)

			rec := doTransferBatchRequest(t, h, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := decodeErrorResponse(t, rec).Error.Code; got != tt.wantCode {
				t.Errorf("error code = %q, want %q", got, tt.wantCode)
			}
			if svc.called != 0 {
				t.Errorf("service was called %d times, want 0: validation must run before the service", svc.called)
			}
		})
	}
}

func TestTransferBatchHandler_PartialFailure(t *testing.T) {
	svc := &fakeService{
		returnErrs: []error{
			nil,
			fmt.Errorf("transfer 1: %w", account.ErrInsufficientFunds),
			nil,
			fmt.Errorf("transfer 3: %w", account.ErrAccountNotFound),
		},
	}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferBatchRequest(t, h, `{"batch":[
		{"from":"a","to":"b","amount":10,"currency":"USD"},
		{"from":"c","to":"d","amount":20,"currency":"USD"},
		{"from":"e","to":"f","amount":30,"currency":"USD"},
		{"from":"g","to":"h","amount":40,"currency":"USD"}
	]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a partially failed batch was still processed", rec.Code)
	}

	resp := decodeBatchResponse(t, rec)
	if resp.Succeeded != 2 || resp.Failed != 2 {
		t.Errorf("succeeded/failed = %d/%d, want 2/2", resp.Succeeded, resp.Failed)
	}

	want := []TransferResultItem{
		{Status: http.StatusOK},
		{Status: http.StatusConflict, Error: &ErrorDetail{Code: "insufficient_funds"}},
		{Status: http.StatusOK},
		{Status: http.StatusNotFound, Error: &ErrorDetail{Code: "account_not_found"}},
	}
	if len(resp.Results) != len(want) {
		t.Fatalf("len(results) = %d, want %d", len(resp.Results), len(want))
	}
	for i, w := range want {
		got := resp.Results[i]
		if got.Status != w.Status {
			t.Errorf("results[%d]: status = %d, want %d", i, got.Status, w.Status)
		}
		switch {
		case w.Error == nil && got.Error != nil:
			t.Errorf("results[%d]: unexpected error %+v", i, got.Error)
		case w.Error != nil && got.Error == nil:
			t.Errorf("results[%d]: error is missing, want code %q", i, w.Error.Code)
		case w.Error != nil && got.Error.Code != w.Error.Code:
			t.Errorf("results[%d]: error code = %q, want %q", i, got.Error.Code, w.Error.Code)
		}
	}
}

func TestTransferBatchHandler_InternalErrorIsNotLeaked(t *testing.T) {
	secret := errors.New("pq: connection to 10.0.0.7 refused")
	svc := &fakeService{returnErrs: []error{secret}}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferBatchRequest(t, h, `{"batch":[{"from":"a","to":"b","amount":10,"currency":"USD"}]}`)

	body := rec.Body.String()
	if strings.Contains(body, secret.Error()) {
		t.Fatalf("response body leaks internal error text: %s", body)
	}

	resp := decodeBatchResponse(t, rec)
	got := resp.Results[0]
	if got.Status != http.StatusInternalServerError || got.Error.Code != "internal" {
		t.Errorf("result = status %d code %q, want status 500 code \"internal\"", got.Status, got.Error.Code)
	}
}

func TestTransferBatchHandler_PassesRequestContext(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/transfers/batch",
		strings.NewReader(`{"batch":[{"from":"a","to":"b","amount":10,"currency":"USD"}]}`))
	marker := "request-scoped"
	req = req.WithContext(context.WithValue(req.Context(), ctxKey, marker))

	h.TransferBatchHandler(httptest.NewRecorder(), req)

	if svc.gotCtx == nil {
		t.Fatal("service received a nil context")
	}
	if got := svc.gotCtx.Value(ctxKey); got != marker {
		t.Errorf("service received context value %v, want %q: the handler must pass r.Context(), not a fresh background context", got, marker)
	}
}

func TestTransferBatchHandler_PropagatesCancellation(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/transfers/batch",
		strings.NewReader(`{"batch":[{"from":"a","to":"b","amount":10,"currency":"USD"}]}`))

	h.TransferBatchHandler(httptest.NewRecorder(), req)

	if svc.gotCtx == nil || svc.gotCtx.Err() == nil {
		t.Error("service received a context that is not cancelled: cancellation must propagate from the request")
	}
}

func TestTransferBatchHandler_TooManyEntries(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	entries := make([]string, maxBatchEntries+1)
	for i := range entries {
		entries[i] = `{"from":"a","to":"b","amount":1,"currency":"USD"}`
	}
	body := fmt.Sprintf(`{"batch":[%s]}`, strings.Join(entries, ","))

	rec := doTransferBatchRequest(t, h, body)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if svc.called != 0 {
		t.Errorf("service was called %d times, want 0", svc.called)
	}
}

func TestMappingFor(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"non positive amount", account.ErrNonPositiveAmount, http.StatusBadRequest, "non_positive_amount"},
		{"same account", account.ErrSameAccount, http.StatusBadRequest, "same_account"},
		{"account not found", account.ErrAccountNotFound, http.StatusNotFound, "account_not_found"},
		{"insufficient funds", account.ErrInsufficientFunds, http.StatusConflict, "insufficient_funds"},
		{"currency mismatch", account.ErrCurrencyMismatch, http.StatusConflict, "currency_mismatch"},
		{"canceled", context.Canceled, statusClientClosedRequest, "canceled"},
		{"deadline exceeded", context.DeadlineExceeded, http.StatusGatewayTimeout, "deadline_exceeded"},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError, "internal"},

		// Обёрнутые ошибки: Service добавляет контекст через %w, и распознавание
		// обязано работать сквозь обёртку.
		{"wrapped insufficient funds", fmt.Errorf("transfer: %w", account.ErrInsufficientFunds), http.StatusConflict, "insufficient_funds"},
		{"struct insufficient funds", &account.InsufficientFundsError{}, http.StatusConflict, "insufficient_funds"},
		{"double wrapped not found", fmt.Errorf("batch: %w", fmt.Errorf("transfer: %w", account.ErrAccountNotFound)), http.StatusNotFound, "account_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mappingFor(tt.err)
			if got.Status != tt.wantStatus {
				t.Errorf("status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
			}
			if got.Message == "" {
				t.Error("message is empty: every mapping must carry a human-readable message")
			}
		})
	}
}

func TestMappingForWrapped(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"non positive amount", account.ErrNonPositiveAmount, http.StatusBadRequest, "non_positive_amount"},
		{"same account", account.ErrSameAccount, http.StatusBadRequest, "same_account"},
		{"account not found", account.ErrAccountNotFound, http.StatusNotFound, "account_not_found"},
		{"insufficient funds", account.ErrInsufficientFunds, http.StatusConflict, "insufficient_funds"},
		{"currency mismatch", account.ErrCurrencyMismatch, http.StatusConflict, "currency_mismatch"},
		{"canceled", context.Canceled, statusClientClosedRequest, "canceled"},
		{"deadline exceeded", context.DeadlineExceeded, http.StatusGatewayTimeout, "deadline_exceeded"},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError, "internal"},
		{"wrapped insufficient funds", fmt.Errorf("transfer: %w", account.ErrInsufficientFunds), http.StatusConflict, "insufficient_funds"},
		{"struct insufficient funds", &account.InsufficientFundsError{}, http.StatusConflict, "insufficient_funds"},
		{"double wrapped not found", fmt.Errorf("batch: %w", fmt.Errorf("transfer: %w", account.ErrAccountNotFound)), http.StatusNotFound, "account_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mappingFor(&account.ServiceError{Err: tt.err})
			if got.Status != tt.wantStatus {
				t.Errorf("status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
			}
			if got.Message == "" {
				t.Error("message is empty: every mapping must carry a human-readable message")
			}
		})
	}
}

// TestErrorMappingsAreUnique guards the mapping table against a copy-paste
// mistake: two entries sharing a code, or a duplicate error, would make one of
// them unreachable, and no other test would notice.
func TestErrorMappingsAreUnique(t *testing.T) {
	seenErr := make(map[error]bool)
	seenCode := make(map[string]bool)
	for _, m := range errorMappings {
		if seenErr[m.err] {
			t.Errorf("duplicate error in the mapping table: %v", m.err)
		}
		if seenCode[m.Code] {
			t.Errorf("duplicate code in the mapping table: %q", m.Code)
		}
		seenErr[m.err] = true
		seenCode[m.Code] = true
	}
	if internalMapping.Code != "internal" {
		t.Errorf("internal mapping code = %q, want \"internal\"", internalMapping.Code)
	}
}

// TestMappingFor_NilPanics fixes the contract of mappingFor: a nil error has no
// HTTP representation. Without this test the contract is only a comment, and a
// caller that consults mappingFor before checking for success would silently
// turn a successful operation into a 500 response.
func TestMappingFor_NilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mappingFor(nil) did not panic: success must be handled before the mapping is consulted")
		}
	}()

	mappingFor(nil)
}

func TestTransferHandler_Success(t *testing.T) {
	h := NewHandler(&fakeService{}, testBatchLimit)

	rec := doTransferRequest(t, h, `{"from":"alice","to":"bob","amount":100,"currency":"USD"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: a successful transfer must not be reported as an error", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("response body = %q, want empty: 204 No Content must carry no body", rec.Body.String())
	}
}

func TestTransferHandler_ArgumentsArePassedToService(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferRequest(t, h, `{"from":"alice","to":"bob","amount":100,"currency":"USD"}`)
	if rec.Code >= http.StatusBadRequest {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	if svc.transferCalled != 1 {
		t.Fatalf("service called %d times, want 1", svc.transferCalled)
	}
	if svc.transferFrom != "alice" || svc.transferTo != "bob" {
		t.Errorf("from/to = %q/%q, want alice/bob: source and destination must not be swapped",
			svc.transferFrom, svc.transferTo)
	}
	want := money.New(100, money.Currency("USD"))
	if svc.transferAmount != want {
		t.Errorf("amount = %+v, want %+v", svc.transferAmount, want)
	}
}

func TestTransferHandler_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{"from":"a",`},
		{"empty body", ``},
		{"not an object", `[1,2,3]`},
		{"unknown field", `{"from":"a","to":"b","amount":10,"currency":"USD","fee":1}`},
		{"empty from", `{"from":"","to":"b","amount":10,"currency":"USD"}`},
		{"empty to", `{"from":"a","to":"","amount":10,"currency":"USD"}`},
		{"empty currency", `{"from":"a","to":"b","amount":10,"currency":""}`},
		{"zero amount", `{"from":"a","to":"b","amount":0,"currency":"USD"}`},
		{"negative amount", `{"from":"a","to":"b","amount":-5,"currency":"USD"}`},
		{"missing fields", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			h := NewHandler(svc, testBatchLimit)

			rec := doTransferRequest(t, h, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if svc.transferCalled != 0 {
				t.Errorf("service was called %d times, want 0: validation must run before the service",
					svc.transferCalled)
			}
		})
	}
}

func TestTransferHandler_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"account not found", account.ErrAccountNotFound, http.StatusNotFound, "account_not_found"},
		{"insufficient funds", account.ErrInsufficientFunds, http.StatusConflict, "insufficient_funds"},
		{"currency mismatch", account.ErrCurrencyMismatch, http.StatusConflict, "currency_mismatch"},
		{"same account", account.ErrSameAccount, http.StatusBadRequest, "same_account"},
		{"non positive amount", account.ErrNonPositiveAmount, http.StatusBadRequest, "non_positive_amount"},
		{"wrapped insufficient funds", fmt.Errorf("transfer: %w", account.ErrInsufficientFunds), http.StatusConflict, "insufficient_funds"},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError, "internal"},
		{"canceled is not exposed as 499", context.Canceled, http.StatusInternalServerError, "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{transferErr: tt.svcErr}
			h := NewHandler(svc, testBatchLimit)

			rec := doTransferRequest(t, h, `{"from":"a","to":"b","amount":10,"currency":"USD"}`)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var resp ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", resp.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestTransferHandler_InternalErrorIsNotLeaked(t *testing.T) {
	secret := errors.New("pq: connection to 10.0.0.7 refused")
	svc := &fakeService{transferErr: secret}
	h := NewHandler(svc, testBatchLimit)

	rec := doTransferRequest(t, h, `{"from":"a","to":"b","amount":10,"currency":"USD"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, secret.Error()) {
		t.Fatalf("response body leaks internal error text: %s", body)
	}
}

func TestTransferHandler_PassesRequestContext(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	req := httptest.NewRequestWithContext(
		context.WithValue(t.Context(), ctxKey, ctxValue),
		http.MethodPost,
		"/transfers",
		strings.NewReader(`{"from":"a","to":"b","amount":10,"currency":"USD"}`),
	)

	h.TransferHandler(httptest.NewRecorder(), req)

	if svc.transferCtx == nil {
		t.Fatal("service received a nil context")
	}
	if got := svc.transferCtx.Value(ctxKey); got != ctxValue {
		t.Errorf("service received context value %v, want %q: the handler must pass r.Context(), not a fresh background context",
			got, ctxValue)
	}
}

func TestTransferHandler_PropagatesCancellation(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/transfers",
		strings.NewReader(`{"from":"a","to":"b","amount":10,"currency":"USD"}`))

	h.TransferHandler(httptest.NewRecorder(), req)

	if svc.transferCtx == nil || svc.transferCtx.Err() == nil {
		t.Error("service received a context that is not cancelled: cancellation must propagate from the request")
	}
}

func TestTransferHandler_BodyTooLarge(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(svc, testBatchLimit)

	padding := strings.Repeat("x", int(maxTransferBodyBytes)+1)
	body := fmt.Sprintf(`{"from":"%s","to":"b","amount":10,"currency":"USD"}`, padding)

	rec := doTransferRequest(t, h, body)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if got := decodeErrorResponse(t, rec).Error.Code; got != "body_too_large" {
		t.Errorf("error code = %q, want body_too_large", got)
	}
	if svc.transferCalled != 0 {
		t.Errorf("service was called %d times, want 0", svc.transferCalled)
	}
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"transfer", http.MethodPost, "/transfers", `{"from":"a","to":"b","amount":10,"currency":"USD"}`, http.StatusNoContent},
		{"batch", http.MethodPost, "/transfers/batch", `{"batch":[{"from":"a","to":"b","amount":10,"currency":"USD"}]}`, http.StatusOK},
		{"transfer wrong method", http.MethodGet, "/transfers", ``, http.StatusMethodNotAllowed},
		{"batch wrong method", http.MethodGet, "/transfers/batch", ``, http.StatusMethodNotAllowed},
		{"unknown path", http.MethodPost, "/nope", ``, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			NewHandler(&fakeService{}, testBatchLimit).Routes(mux)

			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
