// Package forwarder delivers stored Pending punches to the ERP HR service and
// records the outcome. It is the component that makes "we know exactly what has
// and hasn't been delivered" true across restarts: the database is the queue, so
// on boot the forwarder simply resumes draining whatever is still Pending.
package forwarder

import (
	"context"
	"log/slog"
	"time"

	"ismila/erp/zkbio/config"
	batchdb "ismila/erp/zkbio/database/batch_db"
	devicedb "ismila/erp/zkbio/database/device_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
	"ismila/erp/zkbio/pkg/cryptutil"
	"ismila/erp/zkbio/pkg/hrclient"
)

// maxBatchesPerDeviceCycle bounds how many batches one device drains per cycle,
// so a large backlog on one device cannot starve the others.
const maxBatchesPerDeviceCycle = 20

// Forwarder holds the delivery dependencies.
type Forwarder struct {
	cfg    *config.Config
	cipher *cryptutil.Cipher
	hr     *hrclient.Client
	// trigger lets other components (e.g. a fresh push) nudge an immediate flush.
	trigger chan struct{}
}

// New builds a Forwarder.
func New(cfg *config.Config, cipher *cryptutil.Cipher, hr *hrclient.Client) *Forwarder {
	return &Forwarder{cfg: cfg, cipher: cipher, hr: hr, trigger: make(chan struct{}, 1)}
}

// Trigger requests an out-of-band flush (non-blocking; coalesces).
func (f *Forwarder) Trigger() {
	select {
	case f.trigger <- struct{}{}:
	default:
	}
}

// Run drives the delivery loop until ctx is cancelled. On start it flushes
// immediately so a rebooted gateway delivers its backlog without waiting a full
// interval.
func (f *Forwarder) Run(ctx context.Context) {
	slog.Info("forwarder started", "interval", f.cfg.ForwardInterval.String(), "batch", f.cfg.ForwardBatch)
	f.flushAll(ctx)
	ticker := time.NewTicker(f.cfg.ForwardInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("forwarder stopping")
			return
		case <-ticker.C:
			f.flushAll(ctx)
		case <-f.trigger:
			f.flushAll(ctx)
		}
	}
}

// flushAll delivers pending punches for every device that has any.
func (f *Forwarder) flushAll(ctx context.Context) {
	ids, resp := punchdb.DevicesWithPending(f.cfg.MaxAttempts)
	if resp != nil && !resp.IsSuccess {
		slog.Error("forwarder: list pending devices failed", "error", resp.Message)
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		dev, dresp := devicedb.SelectDeviceByID(id)
		if dresp != nil && !dresp.IsSuccess {
			slog.Error("forwarder: load device failed", "device", id, "error", dresp.Message)
			continue
		}
		if dev == nil {
			continue
		}
		f.flushDevice(ctx, dev)
	}
}

// flushDevice drains one device's Pending queue in batches.
func (f *Forwarder) flushDevice(ctx context.Context, dev *devicestructs.ZKDevice) {
	if !dev.HasHRCredential() {
		slog.Warn("forwarder: device has no HR credential — punches held Pending", "device", dev.ID, "name", dev.Name)
		return
	}
	key, err := f.cipher.Decrypt(dev.HRDeviceKeyEnc)
	if err != nil {
		slog.Error("forwarder: cannot decrypt HR key", "device", dev.ID, "error", err.Error())
		return
	}

	for i := 0; i < maxBatchesPerDeviceCycle; i++ {
		if ctx.Err() != nil {
			return
		}
		punches, presp := punchdb.SelectPendingByDevice(dev.ID, f.cfg.ForwardBatch, f.cfg.MaxAttempts)
		if presp != nil && !presp.IsSuccess {
			slog.Error("forwarder: select pending failed", "device", dev.ID, "error", presp.Message)
			return
		}
		if len(punches) == 0 {
			return
		}
		if done := f.deliverBatch(ctx, dev, key, punches); done {
			return // stop draining this device this cycle (error/backoff)
		}
	}
}

// deliverBatch sends one batch and records the result. It returns true when the
// caller should stop draining this device (a failure that warrants backoff).
func (f *Forwarder) deliverBatch(ctx context.Context, dev *devicestructs.ZKDevice, key string, punches []*punchstructs.Punch) (stop bool) {
	logs := make([]hrclient.LogEntry, 0, len(punches))
	ids := make([]string, 0, len(punches))
	for _, p := range punches {
		e := hrclient.LogEntry{
			EmployeeNumber: p.EmployeeNumber,
			LogType:        p.LogType,
			EventTime:      p.EventTime.Format(time.RFC3339),
		}
		if p.Latitude.Valid {
			v := p.Latitude.Float64
			e.Latitude = &v
		}
		if p.Longitude.Valid {
			v := p.Longitude.Float64
			e.Longitude = &v
		}
		logs = append(logs, e)
		ids = append(ids, p.ID)
	}

	now := time.Now()
	resp, err := f.hr.Send(ctx, dev.HRDeviceID, key, logs)

	batch := &batchdb.Batch{DeviceID: dev.ID, HRDeviceID: dev.HRDeviceID, PunchCount: len(ids)}

	// Transport/network error — keep Pending, back off.
	if err != nil {
		batch.Error = err.Error()
		batchID, _ := batchdb.Insert(batch)
		_, _ = punchdb.MarkAttemptFailed(ids, err.Error(), now, f.cfg.MaxAttempts)
		_ = batchID
		slog.Warn("forwarder: delivery failed (will retry)", "device", dev.ID, "count", len(ids), "error", err.Error())
		return true
	}

	batch.HTTPStatus = resp.HTTPStatus
	batch.HRTotal = resp.Result.Total
	batch.HRAccepted = resp.Result.Accepted
	batch.HRIgnored = resp.Result.Ignored
	batch.HRFailed = resp.Result.Failed
	batch.HRDuplicate = resp.Result.Duplicate
	batch.Success = resp.IsSuccess

	// Credential rejected — do not spin; hold Pending, record the error.
	if resp.Auth {
		batch.Error = resp.Message
		_, _ = batchdb.Insert(batch)
		_, _ = punchdb.MarkAttemptFailed(ids, resp.Message, now, f.cfg.MaxAttempts)
		slog.Error("forwarder: HR rejected device credential — fix hrDeviceId/hrDeviceKey", "device", dev.ID, "name", dev.Name)
		return true
	}

	// HR received and durably stored the batch (it stores each raw log first).
	// Accepted/Ignored/Failed are per-punch HR outcomes, but every punch is now
	// on the HR side and a re-send would be a duplicate, so all are Delivered.
	if resp.IsSuccess {
		batchID, _ := batchdb.Insert(batch)
		_, _ = punchdb.MarkSent(ids, batchID, now)
		slog.Info("forwarder: batch delivered", "device", dev.ID, "count", len(ids),
			"accepted", resp.Result.Accepted, "ignored", resp.Result.Ignored,
			"failed", resp.Result.Failed, "duplicate", resp.Result.Duplicate)
		return false // keep draining
	}

	// Definitive non-auth rejection (e.g. 400) — keep Pending, record, back off.
	batch.Error = resp.Message
	_, _ = batchdb.Insert(batch)
	_, _ = punchdb.MarkAttemptFailed(ids, resp.Message, now, f.cfg.MaxAttempts)
	slog.Warn("forwarder: HR rejected batch", "device", dev.ID, "status", resp.HTTPStatus, "message", resp.Message)
	return true
}
