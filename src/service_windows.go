//go:build windows

// Windows Service integration.
//
// The gateway is an appliance: it has to come back on its own after a power cut,
// before anyone logs in. On the Pi that is systemd + Docker restart policies; on
// Windows it is the Service Control Manager, which this file talks to directly
// so no third-party wrapper (NSSM and friends) has to be installed alongside.
//
// Two entry points share this file:
//
//   - The SCM launches zkbio.exe with no arguments in a session with no console.
//     svc.IsWindowsService() detects that and we hand control to svc.Run.
//   - An operator runs `zkbio.exe service <install|uninstall|start|stop|status>`
//     from an elevated prompt to manage the registration.
//
// Anything else (a bare `zkbio.exe` from a console) falls through to the normal
// foreground run in main().
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "zkbio"
	serviceDisplayName = "ZKTeco Biometric Attendance Gateway"
	serviceDescription = "Collects attendance punches from ZKTeco devices on the LAN and forwards them to the ERP HR service. Undelivered punches are queued in the local database and retried."
)

// platformMain intercepts the run when we are either being started by the SCM or
// asked to manage the service registration. handled=false means "not a service
// concern — run in the foreground".
func platformMain() (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return true, fmt.Errorf("cannot determine whether we are a service: %w", err)
	}

	if isService {
		// A service inherits C:\Windows\System32 as its working directory. Both
		// the .env file and the default relative LOG_DIR are resolved from the
		// working directory, so without this the service would start with no
		// config and write its log somewhere surprising.
		if err := chdirToExecutable(); err != nil {
			return true, err
		}
		return true, svc.Run(serviceName, &gatewayService{})
	}

	if len(os.Args) < 2 || os.Args[1] != "service" {
		return false, nil // ordinary console run
	}

	sub := ""
	if len(os.Args) > 2 {
		sub = strings.ToLower(os.Args[2])
	}
	switch sub {
	case "install":
		return true, installService(os.Args[3:])
	case "uninstall", "remove":
		return true, uninstallService()
	case "start":
		return true, startService()
	case "stop":
		return true, stopService()
	case "status":
		return true, statusService()
	default:
		return true, errors.New("usage: zkbio.exe service <install|uninstall|start|stop|status>\n" +
			"       install accepts -depends \"MariaDB\" to wait for a local database service")
	}
}

// ─── SCM handler ────────────────────────────────────────────────────────────

// gatewayService adapts runGateway to the svc.Handler contract: start the
// gateway on a cancellable context, report Running, and cancel it when the SCM
// asks us to stop or the machine shuts down.
type gatewayService struct{}

func (g *gatewayService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	// Best-effort: the event log is the only place an operator can see why a
	// service failed to start, since at boot there is no console and the gateway's
	// own log file may not have been created yet.
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		elog = nil
	} else {
		defer func() { _ = elog.Close() }()
	}
	logInfo := func(msg string) {
		if elog != nil {
			_ = elog.Info(1, msg)
		}
	}
	logError := func(msg string) {
		if elog != nil {
			_ = elog.Error(1, msg)
		}
	}

	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runGateway(ctx) }()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	logInfo(serviceDisplayName + " started")

	for {
		select {
		case err := <-done:
			// The gateway exited on its own — a startup failure (bad config, DB
			// unreachable). Report a non-zero exit code so the SCM's recovery
			// actions kick in and retry.
			changes <- svc.Status{State: svc.StopPending}
			if err != nil {
				logError(serviceDisplayName + " stopped with error: " + err.Error())
				return false, 1
			}
			logInfo(serviceDisplayName + " stopped")
			return false, 0

		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// Give the gateway time to drain in-flight HR deliveries; nothing is
				// lost either way because a punch is only marked Sent after HR
				// confirms it.
				changes <- svc.Status{State: svc.StopPending, WaitHint: 20000}
				cancel()
				<-done
				logInfo(serviceDisplayName + " stopped")
				return false, 0
			}
		}
	}
}

// ─── service management subcommands ─────────────────────────────────────────

func installService(argv []string) error {
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	depends := fs.String("depends", "", "comma-separated services to start first (e.g. \"MariaDB\")")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	exe, err := executablePath()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run this from an elevated prompt): %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	if s, err := m.OpenService(serviceName); err == nil {
		_ = s.Close()
		return fmt.Errorf("service %q is already installed — run 'zkbio.exe service uninstall' first", serviceName)
	}

	cfg := mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
		// Delayed start: at boot the network stack and a local MariaDB are often
		// not ready yet. Combined with the recovery actions below, a gateway that
		// starts too early simply retries instead of staying dead.
		DelayedAutoStart: true,
	}
	if *depends != "" {
		for _, d := range strings.Split(*depends, ",") {
			if d = strings.TrimSpace(d); d != "" {
				cfg.Dependencies = append(cfg.Dependencies, d)
			}
		}
	}

	s, err := m.CreateService(serviceName, exe, cfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer func() { _ = s.Close() }()

	// Restart on failure rather than giving up: an unreachable database at boot
	// must not cost us a day of attendance.
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 120 * time.Second},
	}, 86400); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not set recovery actions:", err)
	}
	// Without this flag the SCM only runs recovery actions when the process dies
	// WITHOUT reporting SERVICE_STOPPED. Our handler reports a clean stop with a
	// non-zero exit code on a startup failure (see Execute), which the SCM
	// treats as "stopped on purpose" and would never retry. This makes a
	// non-zero exit count as a failure too.
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not enable recovery on non-crash failures:", err)
	}

	// Registers the event log source so Execute's messages are readable in Event
	// Viewer. Already-registered is not an error worth failing the install over.
	if err := eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil &&
		!strings.Contains(strings.ToLower(err.Error()), "already exists") {
		fmt.Fprintln(os.Stderr, "warning: could not register event log source:", err)
	}

	fmt.Printf("Installed service %q from %s\n", serviceName, exe)
	fmt.Println("Config is read from .env next to the executable. Start it with:")
	fmt.Println("  zkbio.exe service start")
	return nil
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run this from an elevated prompt): %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", serviceName)
	}
	defer func() { _ = s.Close() }()

	// Stop first so the SCM does not leave the service marked for deletion until
	// the next reboot.
	if st, err := s.Query(); err == nil && st.State != svc.Stopped {
		if _, err := s.Control(svc.Stop); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not stop service before removal:", err)
		} else {
			_ = waitForState(s, svc.Stopped, 30*time.Second)
		}
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	if err := eventlog.Remove(serviceName); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not remove event log source:", err)
	}
	fmt.Printf("Removed service %q\n", serviceName)
	return nil
}

func startService() error {
	s, m, err := openService()
	if err != nil {
		return err
	}
	defer func() { _ = s.Close(); _ = m.Disconnect() }()

	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	if err := waitForState(s, svc.Running, 30*time.Second); err != nil {
		return fmt.Errorf("%w — check Event Viewer and the gateway's logs\\server.log", err)
	}
	fmt.Printf("Service %q is running\n", serviceName)
	return nil
}

func stopService() error {
	s, m, err := openService()
	if err != nil {
		return err
	}
	defer func() { _ = s.Close(); _ = m.Disconnect() }()

	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	if err := waitForState(s, svc.Stopped, 30*time.Second); err != nil {
		return err
	}
	fmt.Printf("Service %q is stopped\n", serviceName)
	return nil
}

func statusService() error {
	s, m, err := openService()
	if err != nil {
		return err
	}
	defer func() { _ = s.Close(); _ = m.Disconnect() }()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service: %w", err)
	}
	fmt.Printf("Service %q: %s\n", serviceName, stateName(st.State))
	return nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func openService() (*mgr.Service, *mgr.Mgr, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to service manager (run this from an elevated prompt): %w", err)
	}
	s, err := m.OpenService(serviceName)
	if err != nil {
		_ = m.Disconnect()
		return nil, nil, fmt.Errorf("service %q is not installed — run 'zkbio.exe service install'", serviceName)
	}
	return s, m, nil
}

func waitForState(s *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("query service: %w", err)
		}
		if st.State == want {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service %q to reach state %s", serviceName, stateName(want))
}

func stateName(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "starting"
	case svc.StopPending:
		return "stopping"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue pending"
	case svc.PausePending:
		return "pause pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown (%d)", s)
	}
}

// executablePath resolves the running binary to an absolute path with symlinks
// expanded — the SCM stores this verbatim and must be able to launch it from any
// working directory.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Abs(exe)
}

func chdirToExecutable() error {
	exe, err := executablePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("change working directory to %s: %w", dir, err)
	}
	return nil
}
