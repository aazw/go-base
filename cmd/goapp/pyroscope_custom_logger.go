package main

import (
	"fmt"
	"strings"
)

// https://pkg.go.dev/github.com/grafana/pyroscope-go#Logger
// https://github.com/grafana/pyroscope-go/blob/v1.2.2/logger.go#L21
// type Logger interface {
//     Infof(_ string, _ ...interface{})
//     Debugf(_ string, _ ...interface{})
//     Errorf(_ string, _ ...interface{})
// }

type PyroscopeCustomLogger struct{}

func (p *PyroscopeCustomLogger) Infof(format string, args ...any) {

	// https://github.com/grafana/pyroscope-go/blob/v1.2.2/session.go#L80-L85
	switch {
	case format == "starting profiling session:":
		return
	case strings.HasPrefix(format, "  AppName:        "):
		format = "starting profiling session: AppName: %+v"
	case strings.HasPrefix(format, "  Tags:           "):
		format = "starting profiling session: Tags: %+v"
	case strings.HasPrefix(format, "  ProfilingTypes: "):
		format = "starting profiling session: ProfilingTypes: %+v"
	case strings.HasPrefix(format, "  DisableGCRuns:  "):
		format = "starting profiling session: DisableGCRuns: %+v"
	case strings.HasPrefix(format, "  UploadRate:     "):
		format = "starting profiling session: UploadRate: %+v"
	}

	logger.Info("pyroscope", "log", fmt.Sprintf(format, args...))
}

func (p *PyroscopeCustomLogger) Debugf(format string, args ...any) {
	logger.Debug("pyroscope", "log", fmt.Sprintf(format, args...))
}

func (p *PyroscopeCustomLogger) Errorf(format string, args ...any) {
	logger.Error("pyroscope", "log", fmt.Sprintf(format, args...))
}
