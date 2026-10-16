package main

import (
	"log/slog"

	"github.com/niclasedge/tollgate/internal/compress"
	"github.com/niclasedge/tollgate/internal/config"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

// withCompress appends the compress stage (#19) when it is on. It runs after
// redact, so it only sees redacted text (ADR-0007).
func withCompress(stages []pipeline.Stage, c config.Compress, r compress.Resolver, logger *slog.Logger) []pipeline.Stage {
	if !c.Enabled {
		return stages
	}
	logger.Info("tollgate: compress stage is on")
	return append(stages, compress.NewStage(r))
}
