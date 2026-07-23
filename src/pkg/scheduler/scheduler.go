// Package scheduler runs the gateway's periodic housekeeping: purging delivered
// punches past their retention window and trimming the delivery-batch audit. It
// never touches Pending or Failed punches, so nothing undelivered is ever
// dropped.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"ismila/erp/zkbio/config"
	batchdb "ismila/erp/zkbio/database/batch_db"
	punchdb "ismila/erp/zkbio/database/punch_db"
)

// StartRetentionPurge runs a daily purge until ctx is cancelled. A no-op when
// retention is zero (keep forever).
func StartRetentionPurge(ctx context.Context, cfg *config.Config) {
	if cfg.PunchRetention <= 0 {
		slog.Info("retention purge disabled (PUNCH_RETENTION_SECONDS=0)")
		return
	}
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		// One purge shortly after boot.
		purge(cfg)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				purge(cfg)
			}
		}
	}()
}

func purge(cfg *config.Config) {
	before := time.Now().Add(-cfg.PunchRetention)
	n, resp := punchdb.PurgeDelivered(before)
	if resp != nil && resp.IsSuccess {
		if n > 0 {
			slog.Info("retention: purged delivered punches", "removed", n, "before", before.Format(time.RFC3339))
		}
	} else if resp != nil {
		slog.Error("retention: purge failed", "error", resp.Message)
	}
	// Keep 180 days of batch audit rows.
	batchdb.PurgeOld(180)
}
