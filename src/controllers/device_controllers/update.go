package devicecontrollers

import (
	"net/http"
	"strings"

	devicedb "ismila/erp/zkbio/database/device_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	"ismila/erp/zkbio/pkg/validate"

	"github.com/gin-gonic/gin"
)

// UpdateDevice applies a partial update to a device.
func UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid id: must be a UUID"))
		return
	}
	var req devicestructs.UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail("Invalid request body: "+err.Error()))
		return
	}
	if req.Mode != nil && !validMode(*req.Mode) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("mode must be one of pull, push, both"))
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

	if _, ur := devicedb.UpdateDevice(id, &req); !ur.IsSuccess {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(ur.Message), "duplicate") {
			status = http.StatusConflict
		}
		c.JSON(status, ur)
		return
	}
	saved, _ := devicedb.SelectDeviceByID(id)
	c.JSON(http.StatusOK, milastructs.Ok("Device updated", devicestructs.ToDeviceResponse(saved)))
}

// SetHRCredential sets or rotates a device's HR credential (X-Device-Id / Key).
func SetHRCredential(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid id: must be a UUID"))
		return
	}
	var req devicestructs.SetHRCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail("Invalid request body: "+err.Error()))
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

	enc, err := cipher.Encrypt(req.HRDeviceKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, milastructs.Err("failed to encrypt HR key"))
		return
	}
	if _, ur := devicedb.SetHRCredential(id, req.HRDeviceID, enc, keyPrefix(req.HRDeviceKey)); !ur.IsSuccess {
		c.JSON(http.StatusInternalServerError, ur)
		return
	}
	saved, _ := devicedb.SelectDeviceByID(id)
	c.JSON(http.StatusOK, milastructs.Ok("HR credential set", devicestructs.ToDeviceResponse(saved)))
}

// DeleteDevice removes a device (its punch history is retained).
func DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid id: must be a UUID"))
		return
	}
	n, resp := devicedb.DeleteDevice(id)
	if !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, milastructs.Fail("device not found"))
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("Device deleted", nil))
}

// RequeueFailed flips a device's Failed punches back to Pending for another
// delivery attempt (used after fixing a credential or an HR-side shift setup).
func RequeueFailed(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid id: must be a UUID"))
		return
	}
	n, resp := punchdb.RequeueFailed(id)
	if !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok("Requeued failed punches", gin.H{"requeued": n}))
}
