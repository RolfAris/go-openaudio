package server

import (
	"io"

	e "github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
)

// ZapEchoLogger adapts a zap.Logger to satisfy echo.Logger.
type ZapEchoLogger zap.Logger

// Compile-time assertion that ZapEchoLogger implements echo.Logger.
var _ e.Logger = (*ZapEchoLogger)(nil)

// helper to get back to *zap.Logger
func (l *ZapEchoLogger) zap() *zap.Logger {
	return (*zap.Logger)(l)
}

func (l *ZapEchoLogger) sugar() *zap.SugaredLogger {
	return l.zap().Sugar()
}

// ---- echo.Logger interface methods ----

// Output-related methods (not used by zap)
func (l *ZapEchoLogger) Output() io.Writer     { return nil }
func (l *ZapEchoLogger) SetOutput(w io.Writer) {}
func (l *ZapEchoLogger) Prefix() string        { return "" }
func (l *ZapEchoLogger) SetPrefix(p string)    {}
func (l *ZapEchoLogger) Level() log.Lvl        { return log.INFO }
func (l *ZapEchoLogger) SetLevel(v log.Lvl)    {}
func (l *ZapEchoLogger) SetHeader(h string)    {}

// Print family
func (l *ZapEchoLogger) Print(i ...interface{}) { l.sugar().Info(i...) }
func (l *ZapEchoLogger) Printf(format string, args ...interface{}) {
	l.sugar().Infof(format, args...)
}
func (l *ZapEchoLogger) Printj(j log.JSON) { l.sugar().Infow("json", "data", j) }

// Debug family
func (l *ZapEchoLogger) Debug(i ...interface{}) { l.sugar().Debug(i...) }
func (l *ZapEchoLogger) Debugf(format string, args ...interface{}) {
	l.sugar().Debugf(format, args...)
}
func (l *ZapEchoLogger) Debugj(j log.JSON) { l.sugar().Debugw("json", "data", j) }

// Info family
func (l *ZapEchoLogger) Info(i ...interface{}) { l.sugar().Info(i...) }
func (l *ZapEchoLogger) Infof(format string, args ...interface{}) {
	l.sugar().Infof(format, args...)
}
func (l *ZapEchoLogger) Infoj(j log.JSON) { l.sugar().Infow("json", "data", j) }

// Warn family
func (l *ZapEchoLogger) Warn(i ...interface{}) { l.sugar().Warn(i...) }
func (l *ZapEchoLogger) Warnf(format string, args ...interface{}) {
	l.sugar().Warnf(format, args...)
}
func (l *ZapEchoLogger) Warnj(j log.JSON) { l.sugar().Warnw("json", "data", j) }

// Error family
func (l *ZapEchoLogger) Error(i ...interface{}) { l.sugar().Error(i...) }
func (l *ZapEchoLogger) Errorf(format string, args ...interface{}) {
	l.sugar().Errorf(format, args...)
}
func (l *ZapEchoLogger) Errorj(j log.JSON) { l.sugar().Errorw("json", "data", j) }

// Fatal family
func (l *ZapEchoLogger) Fatal(i ...interface{}) { l.sugar().Fatal(i...) }
func (l *ZapEchoLogger) Fatalf(format string, args ...interface{}) {
	l.sugar().Fatalf(format, args...)
}
func (l *ZapEchoLogger) Fatalj(j log.JSON) { l.sugar().Fatalw("json", "data", j) }

// Panic family
func (l *ZapEchoLogger) Panic(i ...interface{}) { l.sugar().Panic(i...) }
func (l *ZapEchoLogger) Panicf(format string, args ...interface{}) {
	l.sugar().Panicf(format, args...)
}
func (l *ZapEchoLogger) Panicj(j log.JSON) { l.sugar().Panicw("json", "data", j) }
