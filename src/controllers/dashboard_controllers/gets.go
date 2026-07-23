// Package dashboardcontrollers serves the gateway status overview: how many
// punches are waiting, how many failed, and the health of each device.
package dashboardcontrollers

import (
	"net/http"
	"time"

	devicedb "ismila/erp/zkbio/database/device_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	dashboardstructs "ismila/erp/zkbio/mila_structs/dashboard_structs"

	"github.com/gin-gonic/gin"
)

// onlineWindow is how recently a device must have been seen to count as online.
const onlineWindow = 10 * time.Minute

// Dashboard returns the gateway status overview.
func Dashboard(c *gin.Context) {
	counts, resp := punchdb.CountByStatus()
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	devices, _, dresp := devicedb.ListDevices(1, 200)
	if dresp != nil && !dresp.IsSuccess {
		c.JSON(http.StatusInternalServerError, dresp)
		return
	}

	out := dashboardstructs.DashboardResponse{
		Devices:        len(devices),
		PendingPunches: counts.Pending,
		FailedPunches:  counts.Failed,
		SentPunches:    counts.Sent,
		TotalPunches:   counts.Total,
	}
	if t := punchdb.OldestPending(); t != nil {
		s := t.Format(time.RFC3339)
		out.OldestPending = &s
	}
	if t := punchdb.LastDelivery(); t != nil {
		s := t.Format(time.RFC3339)
		out.LastDelivery = &s
	}

	now := time.Now()
	for _, d := range devices {
		if d.IsActive {
			out.ActiveDevices++
		}
		pending, failed := punchdb.CountPendingFailedForDevice(d.ID)
		h := dashboardstructs.DeviceHealth{
			ID:           d.ID,
			Name:         d.Name,
			SerialNumber: d.SerialNumber,
			Mode:         d.Mode,
			IsActive:     d.IsActive,
			HRCredential: d.HasHRCredential(),
			Pending:      pending,
			Failed:       failed,
		}
		if d.LastSeenAt.Valid {
			s := d.LastSeenAt.Time.Format(time.RFC3339)
			h.LastSeenAt = &s
			h.Online = now.Sub(d.LastSeenAt.Time) <= onlineWindow
		}
		out.DeviceHealth = append(out.DeviceHealth, h)
	}

	c.JSON(http.StatusOK, milastructs.Ok("ok", out))
}
