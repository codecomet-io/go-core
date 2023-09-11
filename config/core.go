package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"go.codecomet.dev/core/filesystem"
	"go.codecomet.dev/core/log"
	"go.codecomet.dev/core/network"
	"go.codecomet.dev/core/reporter"
	"go.codecomet.dev/core/telemetry"
)

func New(appName string, location ...string) *Core {
	conf := &Core{
		location: append([]string{appName}, location...),

		Client: &network.Config{
			TLSMin:              defaultTLSClientMinVersion,
			TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
			DialerKeepAlive:     defaultDialerKeepAlive,
			DialerTimeout:       defaultDialerTimeout,
			DisallowSystemRoot:  false,
			CertPath:            defaultCertPath,
			KeyPath:             defaultKeyPath,
			RootCAs:             []string{},
		},

		Server: &network.Config{
			TLSMin:              defaultTLSServerMinVersion,
			TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
			CertPath:            defaultCertPath,
			KeyPath:             defaultKeyPath,
		},

		Logger: &log.Config{
			Level: defaultLogLevel,
		},
	}

	conf.Client.Resolve = conf.Resolve
	conf.Server.Resolve = conf.Resolve

	// get loglevel override from env
	loglevel := os.Getenv("CODECOMET_LOG_LEVEL")
	loglevel = strings.TrimSpace(loglevel)

	if loglevel != "" {
		switch loglevel {
		case "debug":
			conf.Logger.Level = log.DebugLevel
		case "trace":
			conf.Logger.Level = log.TraceLevel
		default:
			conf.Logger.Level = defaultLogLevel
		}
	}

	return conf
}

type Core struct {
	Reporter  *reporter.Config  `json:"reporter,omitempty"`
	Logger    *log.Config       `json:"logger,omitempty"`
	Telemetry *telemetry.Config `json:"telemetry,omitempty"`
	Client    *network.Config   `json:"client,omitempty"`
	Server    *network.Config   `json:"server,omitempty"`
	location  []string
	Umask     int `json:"umask,omitempty"`
}

func (obj *Core) Trust(ca ...string) {
	if len(ca) > 0 {
		obj.Server.ClientCA = ca[0]
		obj.Client.RootCAs = ca
	}
}

func (obj *Core) Resolve(location ...string) string {
	// Get the absolute path of the containing dir of the config file, resolved against UserConfigDir
	base := absolute(obj.location[:len(obj.location)-1]...)

	// If the desired location is not absolute, resolve against above
	loc := path.Join(location...)
	if !filepath.IsAbs(loc) {
		loc = path.Join(append([]string{base}, location...)...)
	}

	return loc
}

func (obj *Core) Ensure(location ...string) error {
	// Get the absolute path of the containing dir of the config file, resolved against UserConfigDir
	base := absolute(obj.location[:len(obj.location)-1]...)

	loc := path.Join(location...)
	if !filepath.IsAbs(loc) {
		loc = path.Join(append([]string{base}, location...)...)
	}

	err := os.MkdirAll(path.Dir(loc), filesystem.DirPermissionsDefault)
	if err != nil {
		err = fmt.Errorf("failed to ensure parent directory existence for %s: %w", loc, err)
	}

	return err
}

func (obj *Core) OnIO() {
	// Note: calling init everytime we load is not super efficient, but then, how often does that happen?
	// Init filesystem first (capture the current, actual umask before we do anything)
	filesystem.Init()
	// Now, set the umask to whatever
	filesystem.SetUmask(obj.Umask)
}

func (obj *Core) GetLocation() []string {
	return obj.location
}

func (obj *Core) GetDataRoot() string {
	var loc string

	base, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		// XXX figure out impact on iCloud auto backup thing and containers
		loc = path.Join(base, "Library", "Application Support", obj.location[0])
	default:
		loc = path.Join(base, "."+obj.location[0])
	}

	// XXX ignore errors?
	_ = os.MkdirAll(path.Dir(loc), filesystem.DirPermissionsDefault)

	return loc
}

func (obj *Core) GetHome() string {
	home, _ := os.UserHomeDir()

	return home
}

func (obj *Core) GetCacheRoot() string {
	base, _ := os.UserCacheDir()

	loc := path.Join(base, obj.location[0])

	// XXX ignore errors?
	_ = os.MkdirAll(path.Dir(loc), filesystem.DirPermissionsDefault)

	return loc
}

func (obj *Core) GetLogRoot() string {
	var loc string

	base, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		loc = path.Join(base, "Library", "Logs", obj.location[0])
	default:
		loc = "/var/log/" + obj.location[0]
	}

	// XXX ignore errors?
	_ = os.MkdirAll(path.Dir(loc), filesystem.DirPermissionsDefault)

	return loc
}
