package employeecontrollers

import (
	"net/http"
	"strings"
	"time"

	devicedb "ismila/erp/zkbio/database/device_db"
	employeedb "ismila/erp/zkbio/database/employee_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
	"ismila/erp/zkbio/pkg/empsync"
	"ismila/erp/zkbio/pkg/validate"

	"github.com/gin-gonic/gin"

	"ismila/erp/zkbio/config"
)

var cfg *config.Config

// SetDeps injects the gateway config (called once from main), needed for the
// device connect timeout on a manual sync.
func SetDeps(c *config.Config) { cfg = c }

// CreateEmployee adds or corrects one employee by hand. It upserts on
// employeeNumber rather than failing on a known number, because the common case
// is fixing a name a device supplied badly.
func CreateEmployee(c *gin.Context) {
	var req employeestructs.CreateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail("Invalid request body: "+err.Error()))
		return
	}
	number := strings.TrimSpace(req.EmployeeNumber)
	if number == "" {
		c.JSON(http.StatusBadRequest, milastructs.Fail("employeeNumber is required"))
		return
	}

	e := &employeestructs.Employee{
		EmployeeNumber: number,
		Name:           strings.TrimSpace(req.Name),
		CardNumber:     strings.TrimSpace(req.CardNumber),
		Privilege:      req.Privilege,
		IsActive:       true,
	}
	if req.IsActive != nil {
		e.IsActive = *req.IsActive
	}

	outcome, resp := employeedb.UpsertFromOperator(e, employeestructs.SourceManual)
	if outcome == employeedb.Failed {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	stored, _ := employeedb.SelectEmployeeByNumber(number)
	if stored == nil {
		c.JSON(http.StatusOK, milastructs.Ok("employee saved", nil))
		return
	}
	status := http.StatusOK
	msg := "employee updated"
	if outcome == employeedb.Created {
		status = http.StatusCreated
		msg = "employee created"
	}
	c.JSON(status, milastructs.Ok(msg, employeestructs.ToEmployeeResponse(stored)))
}

// UpdateEmployee applies a partial update. Setting the name marks the row
// 'manual' so a later device sync will not undo the correction.
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid employee id"))
		return
	}
	var req employeestructs.UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail("Invalid request body: "+err.Error()))
		return
	}

	n, resp := employeedb.UpdateEmployee(id, &req)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	if n == 0 {
		// Either the id is unknown, or the values were already identical. Resolve
		// it so the caller gets a truthful answer rather than a guess.
		if existing, _ := employeedb.SelectEmployeeByID(id); existing == nil {
			c.JSON(http.StatusNotFound, milastructs.Fail("employee not found"))
			return
		}
	}
	updated, _ := employeedb.SelectEmployeeByID(id)
	if updated == nil {
		c.JSON(http.StatusNotFound, milastructs.Fail("employee not found"))
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("employee updated", employeestructs.ToEmployeeResponse(updated)))
}

// DeleteEmployee removes a registry row. Punches are untouched — they carry the
// employee number themselves and are the record that must survive.
func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid employee id"))
		return
	}
	n, resp := employeedb.DeleteEmployee(id)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, milastructs.Fail("employee not found"))
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("employee deleted", nil))
}

// SyncDeviceEmployees reads one device's enrolled-user table right now, instead
// of waiting for the collector's slow sync interval. Pull-reachable devices only
// — a push device sends its user table to /iclock/cdata on its own schedule.
func SyncDeviceEmployees(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid device id"))
		return
	}
	dev, resp := devicedb.SelectDeviceByID(id)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if dev == nil {
		c.JSON(http.StatusNotFound, milastructs.Fail("device not found"))
		return
	}

	timeout := 10 * time.Second
	if cfg != nil && cfg.PullConnTimeout > 0 {
		timeout = cfg.PullConnTimeout
	}

	result, err := empsync.SyncDevice(dev, timeout)
	if err != nil {
		// Partial results are still worth returning: some users may have been
		// stored before the device dropped the session.
		c.JSON(http.StatusBadGateway, milastructs.AnswerResponse{
			State:     milastructs.FailedState,
			Message:   "employee sync failed: " + err.Error(),
			IsSuccess: false,
			Data:      result,
			Adv:       "none",
		})
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("employees synced from device", result))
}
