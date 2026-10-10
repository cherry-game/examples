// game 节点: 真正的游戏逻辑服, 玩家的业务逻辑都在这里。
//
// 它不直接面对客户端 —— 客户端连的是 gate, gate 再把消息路由过来,
// 所以 isFrontend 是 false, 也不注册任何 connector。
//
// nodeID 必须是纯数字(示例用 10001), 因为它同时被用作雪花算法的节点号,
// 用来避免多个 game 节点生成重复的全局 id。
//
// 可以横向扩展: 再启动一个 nodeID 为 10002 的 game 节点即可, 配置里
// game 的 node_id 写的是正则 "^\\d+$", 表示这一类节点都匹配。
package game

import (
	"github.com/cherry-game/cherry"
	cherrySnowflake "github.com/cherry-game/cherry/extend/snowflake"
	cstring "github.com/cherry-game/cherry/extend/string"
	cherryUtils "github.com/cherry-game/cherry/extend/utils"
	cherryCron "github.com/cherry-game/components/cron"
	cherryGops "github.com/cherry-game/components/gops"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/module/player"
)

// Run 启动 game 节点。
func Run(profileFilePath, nodeID string) {
	// nodeID 既是节点标识, 也是雪花算法的节点号, 所以这里强制要求是数字
	if !cherryUtils.IsNumeric(nodeID) {
		panic("node parameter must is number.")
	}

	// 设置雪花算法使用的节点号。
	// 这样同一个进程内生成的全局 id 不会和其他 game 节点撞车。
	// snowflake global id
	serverId, _ := cstring.ToInt64(nodeID)
	cherrySnowflake.SetDefaultNode(serverId)

	// 配置cherry引擎
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)

	// diagnose
	app.Register(cherryGops.New())
	// 注册调度组件
	app.Register(cherryCron.New())
	// 注册数据配置组件
	app.Register(data.New())
	// 注册检测中心节点组件，确认中心节点启动后，再启动当前节点
	app.Register(checkCenter.New())
	// 注册db组件。示例里没有真正连库, 玩家的数据都在内存里, 重启即还原。
	app.Register(db.New())

	// 注册玩家管理 actor。
	//
	// ActorPlayers 是"玩家总管理", 每个在线玩家再按 playerId 生成一个子 actor:
	//
	//	ActorPlayers       路径 10001.player
	//	└ actorPlayer      路径 10001.player.12345
	//
	// 玩家的具体业务逻辑(查角色/建角色/进游戏/断线)都在子 actor 上, 见 module/player。
	app.AddActors(
		&player.ActorPlayers{},
	)

	app.Startup()
}
