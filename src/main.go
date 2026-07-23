// Command zkbio is the ZKTeco biometric attendance gateway. It runs on a
// Raspberry Pi on the same LAN as the ZKTeco devices, collects attendance
// punches from them (by polling over TCP 4370 and by accepting ADMS/iclock HTTP
// pushes), stores every punch durably with a delivery status, and forwards them
// to the ERP HR service. The local database is the queue, so a reboot loses
// nothing: on boot the gateway resumes delivering whatever is still Pending.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"ismila/erp/zkbio/config"
	devicecontrollers "ismila/erp/zkbio/controllers/device_controllers"
	iclockcontrollers "ismila/erp/zkbio/controllers/iclock_controllers"
	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	"ismila/erp/zkbio/pkg/collector"
	"ismila/erp/zkbio/pkg/cryptutil"
	"ismila/erp/zkbio/pkg/forwarder"
	"ismila/erp/zkbio/pkg/hrclient"
	"ismila/erp/zkbio/pkg/logger"
	"ismila/erp/zkbio/pkg/scheduler"
	"ismila/erp/zkbio/routes"
)

func main() {
	// 1. Environment. A missing .env is fine inside Docker (real env vars).
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, relying on system environment variables")
	}

	cfg, err := config.Load()
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	logger.Setup(logDir, "server.log")
	if err != nil {
		slog.Error("configuration error", "error", err.Error())
		os.Exit(1)
	}
	if cfg.CredEncKey == "" {
		slog.Warn("CRED_ENC_KEY is not set — HR device keys will be stored WITHOUT encryption at rest. Set it in production.")
	}

	// 2. Timezone the gateway interprets naive device timestamps in.
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		slog.Warn("invalid TZ, falling back to UTC", "tz", cfg.TimeZone, "error", err.Error())
		loc = time.UTC
	}

	// 3. Database: open the shared pool and apply the schema (idempotent).
	if err := gendb.Init(cfg.DSN()); err != nil {
		slog.Error("database init failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = gendb.Close() }()
	if err := gendb.Migrate(); err != nil {
		slog.Error("schema migration failed", "error", err.Error())
		os.Exit(1)
	}

	// 4. Crypto for HR keys at rest.
	cipher, err := cryptutil.New(cfg.CredEncKey)
	if err != nil {
		slog.Error("cipher init failed", "error", err.Error())
		os.Exit(1)
	}

	// 5. HR client + forwarder + collector.
	hr := hrclient.New(cfg.HRIngestURL, cfg.HRTimeout, cfg.HRInsecureTLS)
	fwd := forwarder.New(cfg, cipher, hr)

	// 6. Inject controller dependencies.
	devicecontrollers.SetDeps(cipher)
	iclockcontrollers.SetDeps(cfg, loc, fwd.Trigger)

	// 7. Background workers.
	ctx, cancel := context.WithCancel(context.Background())
	go fwd.Run(ctx)
	if cfg.PullEnabled {
		coll := collector.New(cfg, loc, fwd.Trigger)
		go coll.Run(ctx)
	} else {
		slog.Info("pull collector disabled (PULL_ENABLED=false)")
	}
	scheduler.StartRetentionPurge(ctx, cfg)

	// 8. HTTP server.
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, milastructs.Ok("Health check passed", "OK"))
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, milastructs.Ok("ZKTeco biometric gateway", gin.H{
			"service": "zkbio",
			"pull":    cfg.PullEnabled,
			"push":    cfg.PushEnabled,
		}))
	})

	// Any unmatched /iclock/* path is answered OK so firmware variants that call
	// an endpoint we did not register explicitly do not go into a retry storm.
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/iclock") {
			c.String(http.StatusOK, "OK")
			return
		}
		c.JSON(http.StatusNotFound, milastructs.Fail("not found"))
	})

	if cfg.PushEnabled {
		routes.RegisterIClockRoutes(r)
	} else {
		slog.Info("push (iclock) endpoints disabled (PUSH_ENABLED=false)")
	}
	routes.RegisterManagementRoutes(r, os.Getenv("MGMT_API_KEY"))

	srv := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     r,
		ReadTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		slog.Info("starting HTTP server", "addr", srv.Addr, "hrIngest", cfg.HRIngestURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	// 9. Graceful shutdown: stop workers, drain HTTP.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
	cancel() // stop background workers

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("forced shutdown", "error", err.Error())
	}
	slog.Info("server exiting")
}
