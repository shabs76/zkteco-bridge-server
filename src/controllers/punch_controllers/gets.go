// Package punchcontrollers exposes read-only views of stored punches and their
// delivery status — the operator's window into what has and hasn't reached HR.
package punchcontrollers

import (
	"net/http"
	"time"

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
	out := make([]punchstructs.PunchResponse, 0, len(punches))
	for _, p := range punches {
		out = append(out, punchstructs.ToPunchResponse(p))
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
