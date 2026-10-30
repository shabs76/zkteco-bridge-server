package employeestructs

// CreateEmployeeRequest adds or overwrites one employee by hand. Upserts on
// employeeNumber, so re-posting a known number corrects it rather than failing.
type CreateEmployeeRequest struct {
	EmployeeNumber string `json:"employeeNumber" binding:"required"`
	Name           string `json:"name"`
	CardNumber     string `json:"cardNumber"`
	Privilege      int    `json:"privilege"`
	IsActive       *bool  `json:"isActive"`
}

// UpdateEmployeeRequest is a partial update — only non-nil fields are applied.
type UpdateEmployeeRequest struct {
	Name       *string `json:"name"`
	CardNumber *string `json:"cardNumber"`
	Privilege  *int    `json:"privilege"`
	IsActive   *bool   `json:"isActive"`
}
