package main

import (
	"github.com/cherry-game/cherry"
	cherryCluster "github.com/cherry-game/cherry/net/cluster"
)

func main() {
	app := cherry.NewApp(
		"../config/test-discovery.json",
		"master-1",
		true,
		cherry.Cluster,
	)

	app.Register(cherryCluster.New())
	app.Startup()
}
