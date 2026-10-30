// Package punchcontrollers exposes read-only views of stored punches and their
// delivery status — the operator's window into what has and hasn't reached HR.
package punchcontrollers

import (
	"log/slog"
	"net/http"
	"time"

	employeedb "ismila/erp/zkbio/database/employee_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
	"ismila/erp/zkbio/pkg/validate"

	"github.com/gin-gonic/gin"
)

// ListPunches returns a paginated, filterable list of punches (newest first).
// Filters: deviceId, employeeNumber, status (Pending|Sent|Failed), from, to.
func ListPunches(c *gin.Context) {
	page, pageSize := validate.ClampPage(c.Query("page"), c.Query("pageSize"))

	f := punchdb.PunchFilter{
		DeviceID:       c.Query("deviceId"),
		EmployeeNumber: c.Query("employeeNumber"),
		Status:         c.Query("status"),
	}
	if v := c.Query("from"); v != "" {
		if t, ok := parseTime(v); ok {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, ok := parseTime(v); ok {
			f.To = &t
		}
	}

	punches, total, resp := punchdb.ListPunches(f, page, pageSize)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	// Decorate with names from the local registry. One extra query for the whole
	// page, never per row — and a failure here degrades to blank names rather
	// than failing the punch list, which is the operator's core view.
	numbers := make([]string, 0, len(punches))
	seen := map[string]bool{}
	for _, p := range punches {
		if p.EmployeeNumber != "" && !seen[p.EmployeeNumber] {
			seen[p.EmployeeNumber] = true
			numbers = append(numbers, p.EmployeeNumber)
		}
	}
	names, nresp := employeedb.NamesByNumbers(numbers)
	if nresp != nil && !nresp.IsSuccess {
		slog.Warn("punches: name lookup failed, returning punches without names", "error", nresp.Message)
		names = map[string]string{}
	}

	out := make([]punchstructs.PunchResponse, 0, len(punches))
	for _, p := range punches {
		pr := punchstructs.ToPunchResponse(p)
		pr.EmployeeName = names[p.EmployeeNumber]
		out = append(out, pr)
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

func parseTime(s string) (time.Time, bool) {
	for _, l := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
