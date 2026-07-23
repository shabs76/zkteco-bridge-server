package devicecontrollers

import (
	"net/http"

	devicedb "ismila/erp/zkbio/database/device_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	"ismila/erp/zkbio/pkg/validate"

	"github.com/gin-gonic/gin"
)

// ListDevices returns a paginated list of registered devices.
func ListDevices(c *gin.Context) {
	page, pageSize := validate.ClampPage(c.Query("page"), c.Query("pageSize"))
	devices, total, resp := devicedb.ListDevices(page, pageSize)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	out := make([]devicestructs.DeviceResponse, 0, len(devices))
	for _, d := range devices {
		out = append(out, devicestructs.ToDeviceResponse(d))
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

// GetDevice returns a single device by id.
func GetDevice(c *gin.Context) {
	id := c.Param("id")
	if !validate.IsValidUUID(id) {
		c.JSON(http.StatusBadRequest, milastructs.Fail("invalid id: must be a UUID"))
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
	c.JSON(http.StatusOK, milastructs.Ok("ok", devicestructs.ToDeviceResponse(dev)))
}
