package logger

import "go.uber.org/zap"

// L returns the global logger.
func L() *zap.Logger {
	return zap.L()
}

// Init initializes the global logger with production defaults.
func Init() {
	zap.ReplaceGlobals(zap.Must(zap.NewProduction()))
}
