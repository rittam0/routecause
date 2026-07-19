package webapp

import (
	"context"
	"io"
	"log/slog"
	"time"

	"whyfi/internal/collector"
	"whyfi/internal/config"
	"whyfi/internal/model"
	"whyfi/internal/session"
)

type EngineRunner struct {
	Base   config.Config
	Logger *slog.Logger
}

func (r EngineRunner) Run(ctx context.Context, duration time.Duration, observer func(model.ProbeResult)) (model.DiagnosticSession, error) {
	cfg := r.Base
	cfg.Duration = duration
	logger := r.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return session.NewRunner(cfg, collector.RealClock{}, logger).WithObserver(observer).Run(ctx)
}
