package routes

import (
	iclockcontrollers "ismila/erp/zkbio/controllers/iclock_controllers"

	"github.com/gin-gonic/gin"
)

// RegisterIClockRoutes wires the PUBLIC device-facing ADMS/iclock endpoints.
// These are called by the ZKTeco devices themselves over the LAN and are NOT
// behind the management API key — access control is network-level (the gateway
// listens on the LAN the devices are on). They speak plain text, not JSON.
//
// Only the well-known endpoints are registered explicitly; any other /iclock/*
// path a given firmware happens to call is answered with "OK" by the engine's
// NoRoute handler (wired in main), avoiding a router wildcard conflict with these
// static routes.
func RegisterIClockRoutes(r *gin.Engine) {
	g := r.Group("/iclock")
	{
		g.GET("/cdata", iclockcontrollers.CData)  // registration / options handshake
		g.POST("/cdata", iclockcontrollers.CData) // ATTLOG / OPERLOG upload
		g.GET("/getrequest", iclockcontrollers.GetRequest)
		g.POST("/getrequest", iclockcontrollers.GetRequest)
		g.POST("/devicecmd", iclockcontrollers.DeviceCmd)
		g.GET("/ping", iclockcontrollers.Fallback)
		g.POST("/fdata", iclockcontrollers.Fallback)
		g.POST("/edata", iclockcontrollers.Fallback)
		g.GET("/querydata", iclockcontrollers.Fallback)
	}
}
