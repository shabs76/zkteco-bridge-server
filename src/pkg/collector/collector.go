// Package collector runs the pull side: on a schedule it connects to each active
// pull-capable device over TCP 4370, reads the attendance buffer, maps each punch
// to an IN/OUT direction and stores it (idempotently). Stored punches then flow
// to HR via the forwarder.
package collector

import (
	"context"
	"log/slog"
	"time"

	"ismila/erp/zkbio/config"
	devicedb "ismila/erp/zkbio/database/device_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
	"ismila/erp/zkbio/pkg/punchmap"
	"ismila/erp/zkbio/pkg/zk"
)

// Collector polls devices and stores their punches.
type Collector struct {
	cfg     *config.Config
	gwLoc   *time.Location
	onStore func() // called after new punches are stored (nudges the forwarder)
	nextDue map[string]time.Time
}

// New builds a Collector. onStore may be nil.
func New(cfg *config.Config, gwLoc *time.Location, onStore func()) *Collector {
	return &Collector{cfg: cfg, gwLoc: gwLoc, onStore: onStore, nextDue: map[string]time.Time{}}
}

// Run drives the poll loop until ctx is cancelled. The outer tick is fixed and
// short; each device is polled only when its own interval has elapsed.
func (c *Collector) Run(ctx context.Context) {
	slog.Info("pull collector started", "defaultInterval", c.cfg.PullInterval.String())
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Poll once shortly after boot.
	c.cycle(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("pull collector stopping")
			return
		case <-ticker.C:
			c.cycle(ctx)
		}
	}
}

func (c *Collector) cycle(ctx context.Context) {
	devices, resp := devicedb.SelectActiveDevices("pull")
	if resp != nil && !resp.IsSuccess {
		slog.Error("collector: list devices failed", "error", resp.Message)
		return
	}
	now := time.Now()
	stored := 0
	for _, dev := range devices {
		if ctx.Err() != nil {
			return
		}
		if dev.IPAddress == "" {
			continue // pull needs an address
		}
		if due, ok := c.nextDue[dev.ID]; ok && now.Before(due) {
			continue
		}
		interval := c.cfg.PullInterval
		if dev.PollIntervalS > 0 {
			interval = time.Duration(dev.PollIntervalS) * time.Second
		}
		c.nextDue[dev.ID] = now.Add(interval)

		n := c.pollDevice(dev)
		stored += n
	}
	if stored > 0 && c.onStore != nil {
		c.onStore()
	}
}

// pollDevice performs one full pull against a device and stores new punches. It
// returns the number of newly stored punches.
func (c *Collector) pollDevice(dev *devicestructs.ZKDevice) int {
	loc := c.gwLoc
	if dev.Timezone != "" {
		if l, err := time.LoadLocation(dev.Timezone); err == nil {
			loc = l
		}
	}
	port := dev.Port
	if port == 0 {
		port = 4370
	}

	client, err := zk.Dial(dev.IPAddress, port, dev.CommPassword, c.cfg.PullConnTimeout)
	if err != nil {
		devicedb.UpdatePullResult(dev.ID, time.Now(), nil, err.Error())
		slog.Warn("collector: dial failed", "device", dev.ID, "name", dev.Name, "error", err.Error())
		return 0
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		devicedb.UpdatePullResult(dev.ID, time.Now(), nil, err.Error())
		slog.Warn("collector: connect failed", "device", dev.ID, "name", dev.Name, "error", err.Error())
		return 0
	}
	defer client.Disconnect()

	devicedb.TouchLastSeen(dev.ID, time.Now())
	if dev.SerialNumber == "" {
		if sn := client.SerialNumber(); sn != "" {
			devicedb.SetSerialIfEmpty(dev.ID, sn)
			dev.SerialNumber = sn
		}
	}

	records, err := client.ReadAttendance(loc)
	if err != nil {
		devicedb.UpdatePullResult(dev.ID, time.Now(), nil, err.Error())
		slog.Warn("collector: read attendance failed", "device", dev.ID, "name", dev.Name, "error", err.Error())
		return 0
	}

	// Cursor optimization: skip records at or before the newest we've seen, then
	// rely on the DB unique key to collapse any that slip through.
	var cursor time.Time
	if dev.LastCursorTime.Valid {
		cursor = dev.LastCursorTime.Time
	}
	stored := 0
	var maxSeen time.Time
	for _, r := range records {
		if r.Time.After(maxSeen) {
			maxSeen = r.Time
		}
		if !cursor.IsZero() && r.Time.Before(cursor) {
			continue
		}
		np := &punchstructs.NewPunch{
			DeviceID:       dev.ID,
			DeviceSerial:   dev.SerialNumber,
			EmployeeNumber: r.UserID,
			PunchState:     r.State,
			LogType:        punchmap.Map(dev.PunchStateMap, r.State),
			EventTime:      r.Time,
			VerifyMode:     r.VerifyMode,
			Source:         "pull",
			RawPayload:     "",
		}
		outcome, iresp := punchdb.InsertPunch(np)
		switch outcome {
		case punchdb.Inserted:
			stored++
		case punchdb.Duplicate:
			// already have it
		default:
			slog.Error("collector: store punch failed", "device", dev.ID, "error", iresp.Message)
		}
	}

	var cursorPtr *time.Time
	if !maxSeen.IsZero() {
		cursorPtr = &maxSeen
	}
	devicedb.UpdatePullResult(dev.ID, time.Now(), cursorPtr, "")
	if stored > 0 {
		slog.Info("collector: stored punches", "device", dev.ID, "name", dev.Name, "new", stored, "read", len(records))
	}
	return stored
}
