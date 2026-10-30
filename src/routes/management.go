package routes

import (
	dashboardcontrollers "ismila/erp/zkbio/controllers/dashboard_controllers"
	devicecontrollers "ismila/erp/zkbio/controllers/device_controllers"
	employeecontrollers "ismila/erp/zkbio/controllers/employee_controllers"
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
//	POST   /devices/:id/sync-employees    read this device's user table (names) now
//	GET    /punches                       list stored punches + delivery status
//	GET    /employees                     list the local employee/name registry
//	POST   /employees                     add or correct one employee
//	GET    /employees/export              download the registry as CSV
//	POST   /employees/import              upload a CSV (multipart 'file' or raw body)
//	GET    /employees/:id                 get one employee
//	PATCH  /employees/:id                 update one employee
//	DELETE /employees/:id                 remove one employee
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
		api.POST("/devices/:id/sync-employees", employeecontrollers.SyncDeviceEmployees)

		api.GET("/punches", punchcontrollers.ListPunches)

		// Employee (device user) registry. The static /employees/export and
		// /employees/import paths are registered BEFORE /employees/:id so gin
		// matches them literally rather than treating "export" as an :id.
		api.GET("/employees", employeecontrollers.ListEmployees)
		api.POST("/employees", employeecontrollers.CreateEmployee)
		api.GET("/employees/export", employeecontrollers.ExportEmployees)
		api.POST("/employees/import", employeecontrollers.ImportEmployees)
		api.GET("/employees/:id", employeecontrollers.GetEmployee)
		api.PATCH("/employees/:id", employeecontrollers.UpdateEmployee)
		api.DELETE("/employees/:id", employeecontrollers.DeleteEmployee)
	}
}
