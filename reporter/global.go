package reporter

import (
	"github.com/codecomet-io/go-core/log"
	"github.com/codecomet-io/go-core/network"
	"github.com/getsentry/sentry-go"
	"net/http"
	"time"
)

// Init should be called when the app starts, from a config object
func Init(cnf *Config) {
	if cnf.Disabled {
		log.Warn().Msg("Crash reporting and tracing is entirely disabled. This is not recommended.")
		return
	}

	log.Debug().Msg("Initializing crash reporter with config")

	hc := cnf.httpClient
	if hc == nil {
		hc = &http.Client{}
	}

	hc.Transport = network.Get().Transport()

	err := sentry.Init(sentry.ClientOptions{
		HTTPClient:       hc,
		Dsn:              cnf.DSN,
		Environment:      cnf.Environment,
		Release:          cnf.Release,
		Debug:            cnf.Debug,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("sentry.Init failed")
	}
}

func CaptureException(err error) *EventID {
	return sentry.CaptureException(err)
}

func CaptureMessage(msg string) *EventID {
	return sentry.CaptureMessage(msg)
}

func CaptureEvent(e *Event) *EventID {
	return sentry.CaptureEvent(e)
}

func Shutdown() {
	// Flush buffered events before the program terminates.
	// Set the timeout to the maximum duration the program can afford to wait.
	sentry.Flush(2 * time.Second)
}