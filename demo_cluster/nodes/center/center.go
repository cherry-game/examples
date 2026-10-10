// center 节点: 中心服, 处理"全局唯一"的业务。
//
// 典型职责:
//   - 开发账号的注册与查询(demo_cluster/internal/rpc/center 里那几个函数就是调它)
//   - uid 的分配与绑定(同一平台账号无论从哪个网关进来, 都应该是同一个 uid)
//
// 因为要保证全局唯一, 这类业务不能放在可横向扩展的 gate / game 节点上。
// 本例中只能启动一个 center 节点。
package center

import (
	"github.com/cherry-game/cherry"
	cherryCron "github.com/cherry-game/components/cron"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/account"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/ops"
)

// Run 启动 center 节点。
func Run(profileFilePath, nodeID string) {
	app := cherry.Configure(
		profileFilePath,
		nodeID,
		false, // 不接受客户端连接
		cherry.Cluster,
	)

	// 定时任务组件(cron 表达式调度)
	app.Register(cherryCron.New())
	// 策划配表组件: 读取 config/data/ 下的 json 配表, 并提供热重载
	app.Register(data.New())
	// 本节点的 db 组件。示例为了简化, 内部并没有真正连库, 见 nodes/center/db/component.go
	app.Register(db.New())

	// 注册业务 actor。
	//
	//	account  处理账号注册、uid 分配(被 gate / web 通过 rpc 调用)
	//	ops      处理运维指令, check_center 组件就是通过调用它的 ping 来判断 center 是否就绪
	app.AddActors(
		&account.ActorAccount{},
		&ops.ActorOps{},
	)

	app.Startup()
}
