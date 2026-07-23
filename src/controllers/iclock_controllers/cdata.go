// Package iclockcontrollers implements the device-facing ADMS/iclock HTTP
// endpoints. These are called by the ZKTeco devices themselves (not by
// operators), so they speak the plain-text protocol the firmware expects rather
// than the JSON AnswerResponse envelope used by the management API.
package iclockcontrollers

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"ismila/erp/zkbio/config"
	devicedb "ismila/erp/zkbio/database/device_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
	"ismila/erp/zkbio/pkg/iclock"
	"ismila/erp/zkbio/pkg/punchmap"

	"github.com/gin-gonic/gin"
)

var (
	cfg     *config.Config
	gwLoc   *time.Location
	onStore func()
)

// SetDeps injects the shared dependencies (called once from main).
func SetDeps(c *config.Config, loc *time.Location, trigger func()) {
	cfg = c
	gwLoc = loc
	onStore = trigger
}

// CData handles GET (registration handshake) and POST (data upload) on
// /iclock/cdata.
//
// GET  /iclock/cdata?SN=..&options=all   -> option block; keeps the device pushing.
// POST /iclock/cdata?SN=..&table=ATTLOG  -> tab-separated punches; reply "OK".
func CData(c *gin.Context) {
	sn := c.Query("SN")
	if sn == "" {
		c.String(http.StatusOK, "OK")
		return
	}

	if c.Request.Method == http.MethodGet {
		// Registration / options poll. Resolve-or-register so the device is known.
		if cfg != nil && cfg.AutoRegisterSN {
			if id, _ := ensureDevice(sn); id != "" {
				devicedb.TouchLastSeen(id, time.Now())
			}
		}
		c.String(http.StatusOK, iclock.RegistryReply(sn))
		return
	}

	// POST — a data upload. Only ATTLOG carries attendance; everything else
	// (OPERLOG, ATTPHOTO, ...) is acknowledged and ignored.
	table := c.Query("table")
	bodyBytes, _ := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
	if table != "ATTLOG" {
		if id, _ := deviceID(sn); id != "" {
			devicedb.TouchLastSeen(id, time.Now())
		}
		c.String(http.StatusOK, "OK")
		return
	}

	id, dev := ensureDevice(sn)
	if id == "" {
		// Unknown SN and auto-register disabled — acknowledge so the device does
		// not spin, but nothing is stored (visible only in logs).
		slog.Warn("iclock: punch from unregistered device (auto-register off)", "sn", sn)
		c.String(http.StatusOK, "OK")
		return
	}
	devicedb.TouchLastSeen(id, time.Now())

	loc := gwLoc
	stateMap := ""
	if dev != nil {
		if dev.Timezone != "" {
			if l, err := time.LoadLocation(dev.Timezone); err == nil {
				loc = l
			}
		}
		stateMap = dev.PunchStateMap
	}

	records := iclock.ParseAttlog(string(bodyBytes), loc)
	stored := 0
	for _, r := range records {
		np := &punchstructs.NewPunch{
			DeviceID:       id,
			DeviceSerial:   sn,
			EmployeeNumber: r.PIN,
			PunchState:     r.State,
			LogType:        punchmap.Map(stateMap, r.State),
			EventTime:      r.Time,
			VerifyMode:     r.VerifyMode,
			Source:         "push",
			RawPayload:     truncate(r.Raw, 512),
		}
		outcome, _ := punchdb.InsertPunch(np)
		if outcome == punchdb.Inserted {
			stored++
		}
	}
	if stored > 0 {
		slog.Info("iclock: stored pushed punches", "sn", sn, "new", stored, "received", len(records))
		if onStore != nil {
			onStore()
		}
	}
	c.String(http.StatusOK, "OK")
}

// GetRequest answers the device's command poll. We have no commands to push, so
// a bare OK keeps it happy.
func GetRequest(c *gin.Context) {
	if sn := c.Query("SN"); sn != "" {
		if id, _ := deviceID(sn); id != "" {
			devicedb.TouchLastSeen(id, time.Now())
		}
	}
	c.String(http.StatusOK, "OK")
}

// DeviceCmd acknowledges command results posted back by the device.
func DeviceCmd(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// Fallback answers any other /iclock/* chatter with OK.
func Fallback(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// ensureDevice resolves a device by SN, auto-registering an active push device
// row when AutoRegisterSN is on. Returns the id and (best-effort) the row.
func ensureDevice(sn string) (string, *deviceRow) {
	if id, _ := deviceID(sn); id != "" {
		return id, loadRow(id)
	}
	if cfg == nil || !cfg.AutoRegisterSN {
		return "", nil
	}
	id, resp := devicedb.UpsertBySerial(sn, "Auto: "+sn, true)
	if resp != nil && !resp.IsSuccess {
		slog.Error("iclock: auto-register failed", "sn", sn, "error", resp.Message)
		return "", nil
	}
	return id, loadRow(id)
}

// deviceRow is the subset of device fields the push path needs.
type deviceRow struct {
	Timezone      string
	PunchStateMap string
}

func deviceID(sn string) (string, bool) {
	id, _ := devicedb.SelectDeviceIDBySerial(sn)
	return id, id != ""
}

func loadRow(id string) *deviceRow {
	dev, _ := devicedb.SelectDeviceByID(id)
	if dev == nil {
		return nil
	}
	return &deviceRow{Timezone: dev.Timezone, PunchStateMap: dev.PunchStateMap}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
