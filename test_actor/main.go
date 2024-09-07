package main

import (
	"fmt"
	"time"

	"github.com/cherry-game/cherry"
	cherryActor "github.com/cherry-game/cherry/net/actor"
)

func main() {
	fmt.Println("test actor &  child actor")

	app := cherry.Configure(
		"../config/test.json", // 使用环境的配置
		"game-1",              // 使用game-1 的节点id
		false,
		cherry.Standalone,
	)

	system := cherryActor.NewSystem()
	system.SetApp(app)

	parentActor := &actor{}
	system.CreateActor(parentActor.AliasID(), parentActor)

	time.Sleep(1 * time.Hour)
}
