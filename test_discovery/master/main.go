package main

import (
	"github.com/cherry-game/cherry"
)

func main() {
	// master 节点只负责节点发现, 不接收客户端连接, 因此 isFrontend 必须为 false,
	// 否则引擎启动时会因缺少网络包解析器而 panic("net packet parser is nil.")。
	// 使用 cherry.Configure 创建应用, 它会在 Startup 时自动注册 cluster 与 discovery 组件。
	app := cherry.Configure(
		"../config/test-discovery.json",
		"master-1",
		false,
		cherry.Cluster,
	)

	app.Startup()
}
