package main

import (
	_ "embed"

	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"

	"github.com/hanthor/mattermost-matrix-bridge/mattermost"
)

//go:embed example-config.yaml
var ExampleConfig string

type MattermostBridge struct {
	mxmain.BridgeMain
}

func main() {
	br := &MattermostBridge{}
	br.BridgeMain = mxmain.BridgeMain{
		Name:        "mautrix-mattermost",
		Description: "A Matrix-Mattermost puppeting bridge.",
		URL:         "https://github.com/hanthor/mattermost-matrix-bridge",
		Version:     "0.1.0",

		Connector: &mattermost.MattermostConnector{},
	}

	br.Run()
}
