package main

import (
	"maunium.net/go/mautrix/bridgev2/bridgeconfig"
)

type Config struct {
	bridgeconfig.Config `yaml:",inline"`
}

func (c *Config) GetBridge() bridgeconfig.BridgeConfig {
	return c.Bridge
}

func (c *Config) DoCleanup() bool {
	return false
}
