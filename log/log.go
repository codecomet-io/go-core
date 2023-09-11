package log

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init should be called when the app starts, from a config object.
func Init(conf *Config) {
	// This mostly should be the responsibility of the app itself but hey
	zerolog.SetGlobalLevel(conf.Level)
	output := CodecometWriter{Out: os.Stderr, TimeFormat: zerolog.TimeFormatUnix}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

func SetLevel(lv Level) {
	zerolog.SetGlobalLevel(lv)
}

func GetLevel() Level {
	return zerolog.GlobalLevel()
}

func LoggerForLevel(level string) *Event {
	switch level {
  case "trace":
  return log.Trace()
	case "debug":
		return log.Debug()
	case "info":
		return log.Info()
	case "warn":
		return log.Warn()
	case "warning":
		return log.Warn()
	case "error":
		return log.Error()
	case "fatal":
		return log.Fatal()
	default:
		return log.Info()
	}
}

func Fatal() *Event {
	return log.Fatal()
}

func Error() *Event {
	return log.Error()
}

func Warn() *Event {
	return log.Warn()
}

func Info() *Event {
	return log.Info()
}

func Debug() *Event {
	return log.Debug()
}

func Trace() *Event {
	return log.Trace()
}
