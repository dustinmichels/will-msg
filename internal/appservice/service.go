package appservice

import (
	"context"
	"sync"
	"sync/atomic"

	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

// Service coordinates UI operations, parsing, file export, and rule configuration.
type Service struct {
	ctx        context.Context
	engine     atomic.Pointer[engine.RuleEngine]
	rulesDirty atomic.Bool

	recordsMu   sync.Mutex
	lastRecords []engine.Record
	parseGen    uint64

	rulesMu sync.Mutex

	dialogAdapter DialogAdapter
	loadSource    SourceLoader
	revealer      PlatformRevealer
	downloadsDir  DownloadsDirProvider
	loadConfig    ConfigLoader
	saveConfig    ConfigSaver
}

// NewService constructs an initialized Service instance with default configuration and runtime adapters.
func NewService() *Service {
	cfg := config.LoadConfig()
	eng := engine.NewRuleEngine(cfg)

	svc := &Service{
		dialogAdapter: &defaultDialogAdapter{},
		loadSource:    scanner.LoadSource,
		revealer:      defaultPlatformRevealer,
		downloadsDir:  defaultDownloadsDirProvider,
		loadConfig:    config.LoadConfig,
		saveConfig:    config.SaveConfig,
	}
	svc.engine.Store(eng)
	svc.rulesDirty.Store(false)
	return svc
}

// ServiceOptions allows configuring injectable adapters for testing.
type ServiceOptions struct {
	DialogAdapter DialogAdapter
	SourceLoader  SourceLoader
	Revealer      PlatformRevealer
	DownloadsDir  DownloadsDirProvider
	ConfigLoader  ConfigLoader
	ConfigSaver   ConfigSaver
	InitialConfig *config.RuleConfig
}

// NewServiceWithOptions constructs a Service instance with customized adapters (for unit testing).
func NewServiceWithOptions(opts ServiceOptions) *Service {
	cfg := config.DefaultRuleConfig()
	if opts.InitialConfig != nil {
		cfg = opts.InitialConfig.Clone()
	} else if opts.ConfigLoader != nil {
		cfg = opts.ConfigLoader()
	}
	eng := engine.NewRuleEngine(cfg)

	svc := &Service{
		dialogAdapter: opts.DialogAdapter,
		loadSource:    opts.SourceLoader,
		revealer:      opts.Revealer,
		downloadsDir:  opts.DownloadsDir,
		loadConfig:    opts.ConfigLoader,
		saveConfig:    opts.ConfigSaver,
	}
	if svc.dialogAdapter == nil {
		svc.dialogAdapter = &defaultDialogAdapter{}
	}
	if svc.loadSource == nil {
		svc.loadSource = scanner.LoadSource
	}
	if svc.revealer == nil {
		svc.revealer = defaultPlatformRevealer
	}
	if svc.downloadsDir == nil {
		svc.downloadsDir = defaultDownloadsDirProvider
	}
	if svc.loadConfig == nil {
		svc.loadConfig = config.LoadConfig
	}
	if svc.saveConfig == nil {
		svc.saveConfig = config.SaveConfig
	}

	svc.engine.Store(eng)
	svc.rulesDirty.Store(false)
	return svc
}

// Startup saves the Wails runtime context upon application startup.
func (s *Service) Startup(ctx context.Context) {
	s.ctx = ctx
}

// context returns the active Wails context (or context.Background() if uninitialized).
func (s *Service) context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
