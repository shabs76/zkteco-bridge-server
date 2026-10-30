// Command zkbio is the ZKTeco biometric attendance gateway. It runs on a small
// always-on box (a Raspberry Pi, or a Windows machine) on the same LAN as the
// ZKTeco devices, collects attendance punches from them (by polling over TCP
// 4370 and by accepting ADMS/iclock HTTP pushes), stores every punch durably
// with a delivery status, and forwards them to the ERP HR service. The local
// database is the queue, so a reboot loses nothing: on boot the gateway resumes
// delivering whatever is still Pending.
//
// It runs the same on Linux and Windows. On Linux it is a plain foreground
// process (supervised by Docker + systemd); on Windows it additionally knows how
// to run under the Service Control Manager — see service_windows.go.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// Embeds the IANA timezone database in the binary. Windows ships no
	// /usr/share/zoneinfo, so without this time.LoadLocation("Africa/Dar_es_Salaam")
	// fails there and the gateway silently falls back to UTC — every punch would
	// reach HR with a wrong wall-clock time. Go only consults the embedded copy
	// when the host has no tz database of its own, so Linux/Docker behaviour is
	// unchanged.
	_ "time/tzdata"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"ismila/erp/zkbio/config"
	devicecontrollers "ismila/erp/zkbio/controllers/device_controllers"
	employeecontrollers "ismila/erp/zkbio/controllers/employee_controllers"
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
	// Windows only: `zkbio.exe service <install|uninstall|start|stop|status>`
	// manages the SCM registration, and a process launched BY the SCM runs under
	// its control instead of waiting on console signals. On every other OS this
	// is a no-op and we fall through to the foreground path below.
	if handled, err := platformMain(); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "zkbio:", err)
			os.Exit(1)
		}
		return
	}

	// Foreground: run until Ctrl-C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runGateway(ctx); err != nil {
		slog.Error("gateway failed", "error", err.Error())
		os.Exit(1)
	}
}

// runGateway boots the whole gateway and blocks until ctx is cancelled, then
// shuts down gracefully. It returns an error instead of exiting the process so
// the Windows service handler can report the failure to the SCM.
func runGateway(ctx context.Context) error {
	// 1. Environment. A missing .env is fine inside Docker (real env vars).
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, relying on system environment variables")
	}

	cfg, cfgErr := config.Load()
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	if err := logger.Setup(logDir, "server.log"); err != nil {
		return err
	}
	if cfgErr != nil {
		return fmt.Errorf("configuration error: %w", cfgErr)
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
		return fmt.Errorf("database init failed: %w", err)
	}
	defer func() { _ = gendb.Close() }()
	if err := gendb.Migrate(); err != nil {
		return fmt.Errorf("schema migration failed: %w", err)
	}

	// 4. Crypto for HR keys at rest.
	cipher, err := cryptutil.New(cfg.CredEncKey)
	if err != nil {
		return fmt.Errorf("cipher init failed: %w", err)
	}

	// 5. HR client + forwarder + collector.
	hr := hrclient.New(cfg.HRIngestURL, cfg.HRTimeout, cfg.HRInsecureTLS)
	fwd := forwarder.New(cfg, cipher, hr)

	// 6. Inject controller dependencies.
	devicecontrollers.SetDeps(cipher)
	employeecontrollers.SetDeps(cfg)
	iclockcontrollers.SetDeps(cfg, loc, fwd.Trigger)

	// 7. Background workers.
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	go fwd.Run(workerCtx)
	if cfg.PullEnabled {
		coll := collector.New(cfg, loc, fwd.Trigger)
		go coll.Run(workerCtx)
	} else {
		slog.Info("pull collector disabled (PULL_ENABLED=false)")
	}
	scheduler.StartRetentionPurge(workerCtx, cfg)

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

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "addr", srv.Addr, "hrIngest", cfg.HRIngestURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// 9. Graceful shutdown: stop workers, drain HTTP.
	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down...")
	cancelWorkers()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("forced shutdown", "error", err.Error())
	}
	slog.Info("server exiting")
	return nil
}
