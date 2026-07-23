// Package milastructs holds the shared response envelopes and the per-domain
// request/response/model structs. The envelope shapes intentionally mirror the
// ERP HR service (ismila/erp/hrm) so operators and any shared tooling see the
// same JSON contract across the cluster.
package milastructs

// State is the coarse outcome marker carried by every response.
type State string

const (
	// SuccessState — the request succeeded.
	SuccessState State = "success"
	// FailedState — the request was rejected for an expected reason (bad input,
	// not found, unauthorized). The caller can usually fix it and retry.
	FailedState State = "failed"
	// ErrorState — an unexpected server-side error (DB down, etc.).
	ErrorState State = "error"
)

// AnswerResponse is the single-item / mutation envelope. Identical in shape to
// the HR service's AnswerResponse.
type AnswerResponse struct {
	State     State       `json:"state"`
	Message   string      `json:"message"`
	IsSuccess bool        `json:"isSuccess"`
	Data      interface{} `json:"data"`
	Adv       string      `json:"adv"`
}

// Pagination describes the current page of a list response.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
}

// PaginationResponse is the list envelope.
type PaginationResponse struct {
	State      State       `json:"state"`
	Data       interface{} `json:"data"`
	Message    string      `json:"message"`
	Pagination Pagination  `json:"pagination"`
}

// Ok builds a success AnswerResponse.
func Ok(message string, data interface{}) AnswerResponse {
	return AnswerResponse{State: SuccessState, Message: message, IsSuccess: true, Data: data, Adv: "none"}
}

// Fail builds a failed AnswerResponse (expected rejection).
func Fail(message string) AnswerResponse {
	return AnswerResponse{State: FailedState, Message: message, IsSuccess: false, Data: nil, Adv: "none"}
}

// Err builds an error AnswerResponse (unexpected server fault).
func Err(message string) AnswerResponse {
	return AnswerResponse{State: ErrorState, Message: message, IsSuccess: false, Data: nil, Adv: "none"}
}
