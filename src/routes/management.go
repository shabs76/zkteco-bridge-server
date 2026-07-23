package routes

import (
	dashboardcontrollers "ismila/erp/zkbio/controllers/dashboard_controllers"
	devicecontrollers "ismila/erp/zkbio/controllers/device_controllers"
	punchcontrollers "ismila/erp/zkbio/controllers/punch_controllers"
	"ismila/erp/zkbio/pkg/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterManagementRoutes wires the operator-facing JSON API, guarded by the
// management API key.
//
//	GET    /dashboard                     gateway status overview
//	GET    /devices                       list devices
//	POST   /devices                       register a device
//	GET    /devices/:id                   get one device
//	PATCH  /devices/:id                   update a device
//	DELETE /devices/:id                   delete a device
//	POST   /devices/:id/hr-credential     set / rotate the HR credential
//	POST   /devices/:id/requeue           requeue this device's Failed punches
//	GET    /punches                       list stored punches + delivery status
func RegisterManagementRoutes(r *gin.Engine, apiKey string) {
	api := r.Group("/", middleware.APIKey(apiKey))
	{
		api.GET("/dashboard", dashboardcontrollers.Dashboard)

		api.GET("/devices", devicecontrollers.ListDevices)
		api.POST("/devices", devicecontrollers.CreateDevice)
		api.GET("/devices/:id", devicecontrollers.GetDevice)
		api.PATCH("/devices/:id", devicecontrollers.UpdateDevice)
		api.DELETE("/devices/:id", devicecontrollers.DeleteDevice)
		api.POST("/devices/:id/hr-credential", devicecontrollers.SetHRCredential)
		api.POST("/devices/:id/requeue", devicecontrollers.RequeueFailed)

		api.GET("/punches", punchcontrollers.ListPunches)
	}
}
