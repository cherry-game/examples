// master 节点: 集群的"通讯录"。
//
// 它自己不处理任何业务, 职责只有一件事 —— 维护"当前集群里有哪些节点活着":
//
//   - 其他节点启动后向它注册, 它会返回当前的完整节点列表
//   - 节点每秒发一次心跳, 超过心跳超时(默认 3 秒)没收到就把它从列表里移除
//   - 列表有变化时, 广播给所有已注册的节点
//
// 各节点之间靠 nats 通信, 具体地址在 config/demo-cluster.json 的 cluster.nats 段。
// 业务代码拿节点列表的方式是 app.Discovery().ListByType("game") 之类, 见 gate/actor_agent.go。
//
// 启动顺序: master 必须最先启动, 否则其他节点注册不上。
package master

import (
	"github.com/cherry-game/cherry"
)

// Run 启动 master 节点。
//
// 这里没有注册任何组件和 actor —— 集群的发现服务是 cherry.Configure + Cluster 模式下
// 由引擎自动注册的, 业务侧不需要额外代码。
func Run(profileFilePath, nodeID string) {
	// isFrontend 必须是 false。
	// master 不接受客户端连接, 若传 true, 引擎会要求提供网络包解析器, 否则启动时报
	// "net packet parser is nil." 并 panic。
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	app.Startup()
}
