// 网关的消息路由规则。
//
// 客户端发来的每条业务消息都会先进 onPomeloDataRoute, 由它决定"这条消息去哪"。
// 理解这个文件, 基本上就理解了 cherry 集群的消息流转。
package gate

import (
	cslice "github.com/cherry-game/cherry/extend/slice"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
)

var (
	// 客户端连接后，必需先执行第一条协议，进行token验证后，才能进行后续的逻辑
	firstRouteName = "gate.user.login"

	// 角色进入游戏时的前三个协议
	//
	// 这三个协议是"还没绑定角色"时也允许放行的白名单:
	// 玩家必须先用它们完成查角色/建角色/进游戏, 之后 session 里才会有 PlayerID。
	beforeLoginRoutes = []string{
		"game.player.select", //查询玩家角色
		"game.player.create", //玩家创建角色
		"game.player.enter",  //玩家角色进入游戏
	}

	// notLoginRsp 拒绝登录时回给客户端的固定数据(错误码)。
	notLoginRsp = &pb.Int32{
		Value: code.PlayerDenyLogin,
	}
)

// onDataRoute 数据路由规则
//
// 登录逻辑:
// 1.(建立连接)客户端建立连接，服务端对应创建一个agent用于处理玩家消息,actorID == sid
// 2.(用户登录)客户端进行帐号登录验证，通过uid绑定当前sid
// 3.(角色登录)客户端通过'beforeLoginRoutes'中的协议完成角色登录
//
// 与 demo_chat 的 onDataRoute 相比, 这里多了"跨节点转发"的分支:
// route 的第一段是 nodeType, 决定这条消息该由哪种节点处理。
func onPomeloDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	// 每条消息一个 session 快照, 里面带着这条连接的 sid/uid 和 agent.SetData 存下的键值。
	session := agent.NewSession(msg.ID)

	// agent没有"用户登录",且请求不是第一条协议，则踢掉agent，断开连接
	//
	// 这是第一道门禁: 除了登录协议本身, 没有绑定 uid 的连接一律拒绝。
	if !session.IsBind() && msg.Route != firstRouteName {
		agent.Kick(notLoginRsp, true)
		return
	}

	// 比较 route 的第一段(期望的节点类型)与当前节点的类型:
	//
	//	相同 -> 本节点处理。投递到子 actor "节点id.gate.<sid>", 由 ActorAgent 的 login 处理
	//	不同 -> 转发给对应类型的节点, 通常是 game 节点
	if agent.NodeType() == route.NodeType() {
		targetPath := cfacade.NewChildPath(agent.NodeID(), route.HandleName(), session.Sid)
		// cherry v1.6.8 起 LocalDataRoute/ClusterLocalDataRoute 不再接收 session 参数,
		// session 由框架内部按 msg 构建; 这里的 session 仍用于上面的字段读取。
		pomelo.LocalDataRoute(agent, route, msg, targetPath)
	} else {
		gameNodeRoute(agent, session, route, msg)
	}
}

// gameNodeRoute 实现agent路由消息到游戏节点
//
// 负责第二道门禁(角色是否已登录)以及"该发给哪个 game 节点、哪个玩家 actor"。
func gameNodeRoute(agent *pomelo.Agent, session *cproto.Session, route *pmessage.Route, msg *pmessage.Message) {
	// 第一道门禁已经过了(uid 已绑定), 这里再判一次是为了让函数自身也能独立保证前置条件
	if !session.IsBind() {
		return
	}

	// 如果agent没有完成"角色登录",则禁止转发到game节点
	//
	// sessionKey.PlayerID 是 game 节点在玩家进入游戏后, 通过 ActorAgent.setSession
	// 回写到这条连接上的(见 actor_agent.go 的 setSession)。它存在, 说明角色已就绪。
	if !session.Contains(sessionKey.PlayerID) {
		// 如果不是角色登录协议则踢掉agent
		if found := cslice.StringInSlice(msg.Route, beforeLoginRoutes); !found {
			agent.Kick(notLoginRsp, true)
			return
		}
	}

	// ServerID 是玩家登录时选的区服 id, 同时也正好是目标 game 节点的 nodeID。
	// 没有它就不知道该转发给哪个 game 节点。
	serverId := session.GetString(sessionKey.ServerID)
	if serverId == "" {
		return
	}

	// actor 路径是三段式 "game节点id.player.玩家uid":
	//
	//	game 节点的 ActorPlayers actor 收到后, 会按第三段(uid)找到或创建对应的 player 子 actor,
	//	再把消息交给它 —— 详见 nodes/game/module/player/actor_players.go
	childId := cstring.ToString(session.Uid)
	targetPath := cfacade.NewChildPath(serverId, route.HandleName(), childId)

	// 与 LocalDataRoute 的区别: 这会通过 nats 把消息投递给另一个进程。
	pomelo.ClusterLocalDataRoute(agent, route, msg, serverId, targetPath)
}
