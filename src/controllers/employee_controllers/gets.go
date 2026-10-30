// Package employeecontrollers exposes the local employee (device user) registry:
// the names the gateway has learned from its devices, plus the operator's own
// corrections and CSV import/export.
//
// Scope note: names live here and are NOT forwarded to HR. The HR ingest
// contract (pkg/hrclient.LogEntry) carries employeeNumber only, and this gateway
// keeps that contract in lock-step with HR's device_structs. The registry exists
// so an operator can tell WHO a punch belongs to while reconciling.
package employeecontrollers

import (
	"net/http"

	employeedb "ismila/erp/zkbio/database/employee_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
	"ismila/erp/zkbio/pkg/validate"

	"github.com/gin-gonic/gin"
)

// ListEmployees returns a paginated, filterable list ordered by employee number.
// Filters: q (number or name), source, deviceId, active.
func ListEmployees(c *gin.Context) {
	page, pageSize := validate.ClampPage(c.Query("page"), c.Query("pageSize"))

	f := employeedb.EmployeeFilter{
		Search:   c.Query("q"),
		Source:   c.Query("source"),
		DeviceID: c.Query("deviceId"),
		Active:   c.Query("active"),
	}

	employees, total, resp := employeedb.ListEmployees(f, page, pageSize)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	out := make([]employeestructs.EmployeeResponse, 0, len(employees))
	for _, e := range employees {
		out = append(out, employeestructs.ToEmployeeResponse(e))
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	c.JSON(http.StatusOK, milastructs.PaginationResponse{
		State:   milastructs.SuccessState,
		Data:    out,
		Message: "ok",
		Pagination: milastructs.Pagination{
			Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
		},
	})
}

// GetEmployee returns one employee by id.
func GetEmployee(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid employee id"))
		return
	}
	e, resp := employeedb.SelectEmployeeByID(id)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if e == nil {
		c.JSON(http.StatusNotFound, milastructs.Fail("employee not found"))
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("ok", employeestructs.ToEmployeeResponse(e)))
}
