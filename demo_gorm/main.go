// 这是 cherry 接入 MySQL 的最小示例, 演示三件事:
//
//  1. 如何注册并使用 gorm 组件连数据库
//  2. 如何从节点的私有配置(__settings__)里读取参数
//  3. 如何在 actor 里挂定时器, 周期性执行逻辑
//
// 运行前请先按 demo_gorm/README.md 导入 db.sql 并确认 config/demo-gorm.json 里的
// 数据库地址/账号/密码正确。
//
// 这个示例是 Standalone(单节点)模式, 不需要 nats。
package main

import (
	"github.com/cherry-game/cherry"
	cherryGORM "github.com/cherry-game/components/gorm"
)

func main() {
	// 创建应用并读取 profile 配置。参数依次为:
	//
	//	path        配置文件路径
	//	nodeID      本节点 id, 这里用 "game-1"
	//	isFrontend  是否前端节点。本示例不接收客户端连接, 所以是 false
	//	nodeMode    Standalone(单节点) / Cluster(多节点)
	//
	// 注意 nodeID 必须能在配置文件的 node 段里找到同名的条目,
	// 因为节点的 address、__settings__ 等都是按 nodeID 查出来的。
	app := cherry.Configure(
		"../config/demo-gorm.json", // 使用环境的配置
		"game-1",                   // 使用game-1 的节点id
		false,
		cherry.Standalone,
	)

	// 注册 gorm 组件。
	//
	// 组件(component)是引擎的可插拔单元, 有 Init / OnAfterInit / OnStop 等生命周期。
	// 注册之后:
	//   - 引擎启动时会自动按配置连好所有数据库
	//   - 业务代码可以按名字把它取出来用: p.App().Find(cherryGORM.Name)
	//
	// 具体连哪些库、账号密码是什么, 全在 config/demo-gorm.json 的 "db" 段里。
	app.Register(cherryGORM.NewComponent())

	// 注册业务 actor。
	// actor 是引擎的最小并发单元, 每个 actor 有独立 goroutine 和消息队列,
	// 内部字段因此不需要加锁。这里注册的 ActorDB 负责定时查库, 见 actor_gorm.go。
	app.AddActors(
		&ActorDB{},
	)

	// 启动引擎并阻塞在这里, 直到收到退出信号。
	// 组件初始化 -> 组件 OnAfterInit -> actor 启动 -> 进入运行状态。
	app.Startup()
}
