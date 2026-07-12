package api

// TransferRequest is the JSON body accepted by TransferHandler.
//
// All fields are required. Amount is expressed in minor units and must be
// strictly positive. Unknown fields are rejected, so a misspelled field name is
// reported to the caller instead of being silently ignored.
type TransferRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// TransferBatchRequest is the JSON body accepted by TransferBatchHandler.
//
// Batch must contain at least one entry. Entries are executed independently;
// the order of entries is preserved in the response.
type TransferBatchRequest struct {
	Batch []TransferRequest `json:"batch"`
}

// TransferBatchResponse is the JSON body returned by TransferBatchHandler when
// the batch was processed.
//
// Results has the same length and order as the Batch field of the request, so
// the caller can match every outcome to the transfer that produced it.
// Succeeded and Failed are derived from Results and are provided so that a
// caller can detect partial failure without scanning the list.
type TransferBatchResponse struct {
	Results   []TransferResultItem `json:"results"`
	Succeeded int                  `json:"succeeded"`
	Failed    int                  `json:"failed"`
}

// TransferResultItem is the outcome of a single transfer inside a batch.
//
// Status is 2xx if the transfer was applied and 4xx or 5xx otherwise; it uses
// HTTP status codes so that the meaning of an entry outcome matches the meaning
// of the same code on the single-transfer endpoint. Error is empty on success.
type TransferResultItem struct {
	Status int          `json:"status"`
	Error  *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail describes a failure in a machine-readable way.
//
// Code is stable and may be branched on by clients; Message is human-readable
// and may change without notice. Internal error text is never exposed here.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the JSON body returned with every 4xx and 5xx response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
