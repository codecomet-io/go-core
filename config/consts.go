package config

import (
	"crypto/tls"
	"time"

	"go.codecomet.dev/core/filesystem"
	"go.codecomet.dev/core/log"
)

const (
	defaultDirPerms            = filesystem.DirPermissionsDefault
	defaultFilePerms           = filesystem.FilePermissionsDefault
	defaultLogLevel            = log.InfoLevel
	defaultTLSClientMinVersion = tls.VersionTLS12
	defaultTLSServerMinVersion = tls.VersionTLS13
	defaultDialerKeepAlive     = 30 * time.Second
	defaultDialerTimeout       = 30 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
	defaultCertPath            = "x509.crt"
	defaultKeyPath             = "x509.key"
)
