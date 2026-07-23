// Package devicecontrollers holds the operator-facing device registry handlers.
// They bind JSON, validate, call the *_db layer, and return the shared
// AnswerResponse envelope — the same layering the HR service uses.
package devicecontrollers

import (
	"net/http"
	"strings"

	devicedb "ismila/erp/zkbio/database/device_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	"ismila/erp/zkbio/pkg/cryptutil"

	"github.com/gin-gonic/gin"
)

var cipher *cryptutil.Cipher

// SetDeps injects the credential cipher (called once from main).
func SetDeps(c *cryptutil.Cipher) { cipher = c }

func validMode(m string) bool {
	switch m {
	case "pull", "push", "both":
		return true
	}
	return false
}

// keyPrefix returns a short, non-reconstructable display prefix of a secret.
func keyPrefix(secret string) string {
	if len(secret) <= 8 {
		return secret
	}
	return secret[:8]
}

// CreateDevice registers a physical ZKTeco device with the gateway.
func CreateDevice(c *gin.Context) {
	var req devicestructs.CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail("Invalid request body: "+err.Error()))
		return
	}

	if req.Mode == "" {
		req.Mode = "both"
	}
	if !validMode(req.Mode) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("mode must be one of pull, push, both"))
		return
	}
	if req.Port == 0 {
		req.Port = 4370
	}
	if (req.Mode == "pull" || req.Mode == "both") && strings.TrimSpace(req.IPAddress) == "" {
		c.JSON(http.StatusBadRequest, milastructs.Fail("ipAddress is required for pull mode"))
		return
	}
	if (req.Mode == "push") && strings.TrimSpace(req.SerialNumber) == "" {
		c.JSON(http.StatusBadRequest, milastructs.Fail("serialNumber is required for push mode (the device identifies itself by SN)"))
		return
	}

	dev := &devicestructs.ZKDevice{
		Name:          req.Name,
		SerialNumber:  req.SerialNumber,
		IPAddress:     req.IPAddress,
		Port:          req.Port,
		CommPassword:  req.CommPassword,
		Mode:          req.Mode,
		Timezone:      req.Timezone,
		PunchStateMap: req.PunchStateMap,
		PollIntervalS: req.PollIntervalS,
		IsActive:      true,
	}

	// Encrypt the HR key at rest if one was supplied.
	if strings.TrimSpace(req.HRDeviceID) != "" && strings.TrimSpace(req.HRDeviceKey) != "" {
		enc, err := cipher.Encrypt(req.HRDeviceKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, milastructs.Err("failed to encrypt HR key"))
			return
		}
		dev.HRDeviceID = req.HRDeviceID
		dev.HRDeviceKeyEnc = enc
		dev.HRKeyPrefix = keyPrefix(req.HRDeviceKey)
	}

	id, resp := devicedb.InsertDevice(dev)
	if !resp.IsSuccess {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(resp.Message), "duplicate") {
			status = http.StatusConflict
		}
		c.JSON(status, resp)
		return
	}

	saved, _ := devicedb.SelectDeviceByID(id)
	c.JSON(http.StatusCreated, milastructs.Ok("Device registered", devicestructs.ToDeviceResponse(saved)))
}
