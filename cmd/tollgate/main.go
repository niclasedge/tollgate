// Command tollgate is the Claude API gateway: it authenticates clients, runs
// every request through the pipeline and forwards it to CLIProxyAPI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tollgate "github.com/niclasedge/tollgate"
	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/auth"
	"github.com/niclasedge/tollgate/internal/config"
	"github.com/niclasedge/tollgate/internal/cryptobox"
	"github.com/niclasedge/tollgate/internal/dashboard"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/redactcfg"
	"github.com/niclasedge/tollgate/internal/server"
	"github.com/niclasedge/tollgate/internal/store"
	"github.com/niclasedge/tollgate/internal/upstream"
	"github.com/niclasedge/tollgate/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run, args := serve, os.Args[1:]
	if len(args) > 0 && args[0] == "notify" {
		run, args = notify, args[1:]
	}
	if err := run(ctx, args, os.Stderr, os.LookupEnv); err != nil {
		stop()
		os.Exit(1)
	}
}

// serve wires configuration, store, pipeline and server and blocks until ctx is
// done. Every failure is logged as one JSON error line and returned.
func serve(ctx context.Context, args []string, stderr io.Writer, getenv func(string) (string, bool)) error {
	level := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level}))
	fail := func(err error) error {
		logger.Error("tollgate: start failed", "error", err.Error())
		return err
	}

	fs := flag.NewFlagSet("tollgate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	confPath := fs.String("config", "", "path to tollgate.yaml")
	dev := fs.Bool("dev-allow-any-listen", false, "development only: allow a listen address outside the Tailnet ranges")
	if err := fs.Parse(args); err != nil {
		return fail(fmt.Errorf("arguments: %w", err))
	}

	cfg, err := config.Load(*confPath, getenv)
	if err != nil {
		return fail(err)
	}
	if err := cfg.Validate(*dev); err != nil {
		return fail(err)
	}
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(strings.ToLower(cfg.Log.Level))); err == nil {
		level.Set(lv)
	}
	if *dev {
		logger.Warn("tollgate: --dev-allow-any-listen is set, the Tailnet binding check is lifted")
	}

	cpaKey, err := config.ReadSecretFile(cfg.Upstream.APIKeyFile)
	if err != nil {
		return fail(err)
	}
	var mgmtKey string
	if cfg.Upstream.ManagementKeyFile != "" {
		if mgmtKey, err = config.ReadSecretFile(cfg.Upstream.ManagementKeyFile); err != nil {
			return fail(err)
		}
	}
	st, err := store.Open(ctx, cfg.DB.Path, store.Options{})
	if err != nil {
		return fail(fmt.Errorf("open store: %w", err))
	}
	defer st.Close()

	// One vault key seals account tokens (#28) and the word list (#15). A new
	// key is only created while no account depends on an existing one.
	records, err := st.ListAccounts(ctx)
	if err != nil {
		return fail(fmt.Errorf("read account vault: %w", err))
	}
	var box *cryptobox.Box
	var sealer redactcfg.Sealer
	if cfg.Crypto.KeyFile != "" {
		if box, err = cryptobox.Open(cfg.Crypto.KeyFile, len(records) == 0); err != nil {
			return fail(err)
		}
		sealer = box
	}
	// Versioned runtime configuration (#15). Opened before the pipeline so the
	// first start activates version 1 and sessions pin it from the start.
	runtimeConfig, err := redactcfg.Open(ctx, st, redactcfg.Options{Sealer: sealer, Fixtures: tollgate.Fixtures(), Logger: logger,
		CompressEnabled: cfg.Compress.Enabled})
	if err != nil {
		return fail(fmt.Errorf("open runtime configuration: %w", err))
	}

	// Stage 0 pins the session's configuration version; redact (#13) and
	// compress (#19) use the pinned version, the return path (#14) runs in
	// the server.
	stages := []pipeline.Stage{&pipeline.Intake{Keys: auth.NewVerifier(st), Pinner: &pipeline.SessionPinner{Store: st}}}
	if cfg.Redact.Enabled {
		stages = append(stages, redact.NewStage(runtimeConfig))
		logger.Info("tollgate: redact stage is on")
	}
	stages = withCompress(stages, cfg.Compress, runtimeConfig, logger)
	runner := pipeline.NewRunner(stages)
	adminToken, err := auth.LoadOrCreateAdminToken(cfg.Admin.TokenFile)
	if err != nil {
		return fail(err)
	}
	// One admin handler for /api/: client keys and dashboard, admin token only.
	broker := dashboard.NewBroker()
	limits := meter.NewLimitTracker(nil)
	accountGate := new(accounts.Gate)
	var accountManager *accounts.Manager
	if mgmtKey != "" {
		if box == nil {
			return fail(errors.New("account vault needs crypto.key_file"))
		}
		accountUp, err := upstream.New(cfg.Upstream.URL)
		if err != nil {
			return fail(err)
		}
		accountManager = accounts.New(st, box, accountUp, mgmtKey, cpaKey, accountGate)
		accountManager.OnSwitch = limits.Reset
		initCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		if err := accountManager.Initialize(initCtx); err != nil {
			logger.Warn("account initialization deferred until next proxy request")
		}
		cancel()
		startAlerts(ctx, cfg.Alerts, accountManager, broker, logger)
	}
	// srv is set below; the speedtest only runs once the server serves.
	var srv *server.Server
	probe := func(ctx context.Context, model string, skipRedact bool) (string, error) {
		return srv.Probe(ctx, server.ProbeInput{Body: server.SpeedtestBody(model), SkipRedact: skipRedact})
	}
	dash, err := dashboard.New(dashboard.Options{
		Accounts: accountManager, Store: st, Broker: broker, UpstreamURL: cfg.Upstream.URL, ManagementKey: mgmtKey, Logger: logger, Done: ctx.Done(),
		Limits: limits.Latest, Probe: probe, RedactEnabled: cfg.Redact.Enabled,
	})
	if err != nil {
		return fail(err)
	}
	adminMux := http.NewServeMux()
	auth.RegisterClientRoutes(adminMux, st, logger)
	dash.Register(adminMux)
	runtimeConfig.Register(adminMux)
	pub := dashboard.NewPublisher(broker, st)
	defer pub.Close()
	var accountReady func(context.Context) error
	var accountID func(context.Context) (string, error)
	var accountResponseObserver func(string, http.Header)
	if accountManager != nil {
		accountReady = accountManager.Initialize
		accountID = accountManager.ActiveID
		accountResponseObserver = accountManager.ObserveUsage
	}
	srv, err = server.New(server.Options{
		AccountGate: accountGate, AccountReady: accountReady, AccountID: accountID, Runner: runner, Store: st, UpstreamURL: cfg.Upstream.URL, UpstreamKey: cpaKey, Logger: logger,
		Admin:                   auth.RequireAdmin(adminToken, adminMux),
		TraceObserver:           pub.Observe,
		ResponseObserver:        limits.Observe,
		AccountResponseObserver: accountResponseObserver,
		Frontend:                web.Handler(),
	})
	if err != nil {
		return fail(err)
	}
	ln, err := server.Listen(cfg.Listen, cfg.Published, *dev)
	if err != nil {
		return fail(err)
	}

	// An unreachable CLIProxyAPI is not fatal (it may start later); warn only.
	if up, err := upstream.New(cfg.Upstream.URL); err == nil {
		hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := up.Health(hctx); err != nil {
			logger.Warn("tollgate: upstream health check failed", "error", err.Error())
		}
		cancel()
	}

	logger.Info("tollgate: listening", "addr", ln.Addr().String(), "published", cfg.Published, "admin_token_file", cfg.Admin.TokenFile)
	if err := srv.Serve(ctx, ln); err != nil {
		return fail(err)
	}
	logger.Info("tollgate: stopped")
	return nil
}
