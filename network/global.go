package network

import (
	"github.com/codecomet-io/go-core/log"
	"net/http"
)

var network *Network

// Init should be called when the app starts, from config objects
func Init(client *Config, server *Config) {
	log.Debug().Msg("Initializing network core with config")

	network = &Network{
		clientConfig: client,
		serverConfig: server,
	}

	http.DefaultTransport = network.Transport()
}

// Get returns the network instance, from which a new Transport or TLSConfig object can be retrieved
func Get() *Network {
	return network
}