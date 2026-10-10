package main

import (
	"github.com/cherry-game/cherry"
)

func main() {
	// 使用 cherry.Configure 创建应用: 它在 Startup 时会自动注册 cluster 与 discovery 组件,
	// 节点启动后会向 master(master-1) 注册并拿到全部成员列表。
	app := cherry.Configure(
		"../config/test-discovery.json",
		"game-2",
		false,
		cherry.Cluster,
	)

	app.Startup()
}
