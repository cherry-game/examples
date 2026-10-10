// gate 节点: 网关, 唯一直接面对客户端连接的业务节点。
//
// 它承担三件事:
//
//	连接管理  维护每个客户端连接的 Agent, 断线时清理
//	登录校验  校验 token, 绑定 uid, 处理重复登录(顶号)
//	消息转发  按 route 决定消息留在本地处理, 还是转发给 game 节点
//
// 监听两个端口:
//
//	:10011  tcp        给 robot_client 压测机器人用
//	:10010  websocket  给 h5 客户端用(取自配置里本节点的 address)
//
// 可以横向扩展: 再起一个 nodeID 为 gc-gate-2 的实例即可, 配置里 gate 的
// node_id 写的是正则 "^gc-gate-\\d+$"。多个网关之间的顶号靠 checkGateSession 互相通知。
package gate

import (
	"encoding/binary"
	"time"

	"github.com/cherry-game/cherry"
	cfacade "github.com/cherry-game/cherry/facade"
	cconnector "github.com/cherry-game/cherry/net/connector"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	"github.com/cherry-game/cherry/net/parser/simple"
	cproto "github.com/cherry-game/cherry/net/proto"
	cherryGops "github.com/cherry-game/components/gops"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
)

// Run 运行gate节点
// gate 主要用于对外提供网络连接、管理用户连接、消息转发
func Run(profileFilePath, nodeID string) {
	// 创建一个cherry实例。isFrontend 为 true: 只有前端节点才能挂连接器接收客户端连接。
	app := cherry.Configure(
		profileFilePath,
		nodeID,
		true,
		cherry.Cluster,
	)

	// 预计算"重复登录"的踢人包。
	// 该值只取决于固定的错误码, 所有连接的结果都一样, 所以节点启动时算一次即可。
	// 原先是在每个 ActorAgent 的 OnInit() 里给这个包级变量赋值,
	// 而每个客户端连接都会并发创建 ActorAgent, 于是产生对同一变量的并发写(数据竞争,
	// 用 `go build -race` 可以复现)。
	duplicateLoginCode, _ = app.Serializer().Marshal(&cproto.I32{
		Value: code.PlayerDuplicateLogin,
	})

	// 设置网络数据包解析器
	netParser := buildPomeloParser(app)
	//netParser := buildSimpleParser(app)
	app.SetNetParser(netParser)

	app.Register(cherryGops.New())
	// 注册检则中心服组件，用于检则中心服是否先启动
	app.Register(checkCenter.New())
	// 注册数据配表组件，具体详见data-config的使用方法和参数配置
	app.Register(data.New())

	//启动cherry引擎
	app.Startup()
}

// buildPomeloParser 构建 pomelo 协议的解析器(示例实际使用的方案)。
//
// pomelo 是网易开源的游戏网络协议, h5 客户端和压测机器人用的都是它。
// "解析器"(INetParser)负责: 监听端口 -> 收发数据包 -> 解码成 route + 消息体 -> 交给路由函数。
func buildPomeloParser(app *cherry.AppBuilder) cfacade.INetParser {
	// 使用pomelo网络数据包解析器。
	// "user" 既是该解析器 actor 的 actorID, 也是每条连接子 actor 的父节点名。
	agentActor := pomelo.NewActor("user")
	//创建一个tcp监听，用于client/robot压测机器人连接网关tcp
	agentActor.AddConnector(cconnector.NewTCP(":10011"))
	//再创建一个websocket监听，用于h5客户端建立连接
	agentActor.AddConnector(cconnector.NewWS(app.Address()))
	//当有新连接创建Agent时，启动一个自定义(ActorAgent)的子actor
	agentActor.SetOnNewAgent(func(newAgent *pomelo.Agent) {
		childActor := &ActorAgent{}
		// 连接断开时回调, 由子 actor 负责通知 game 节点关闭对应的玩家会话
		newAgent.AddOnClose(childActor.onSessionClose)
		// actorID == sid, 于是每个连接的子 actor 路径是 "<节点id>.user.<sid>"
		agentActor.Child().Create(newAgent.SID(), childActor)
	})

	// 设置数据路由函数
	agentActor.SetOnDataRoute(onPomeloDataRoute)

	return agentActor
}

// buildSimpleParser 构建简单的网络数据包解析器(备用方案, 当前未启用)
//
// 这是 cherry simple 解析器的用法参考, 需要时把 Run() 中的
// `netParser := buildPomeloParser(app)` 换成 `buildSimpleParser(app)` 即可。
// 与 pomelo 的区别: simple 走的是"消息号(mid)"而不是 route 字符串,
// 需要先用 AddNodeRoute 把消息号映射到目标 actor 和函数上。
//
// 注意: 自定义路由请实现 `SetOnDataRoute(fn)` 并传入自己的路由函数,
// 这里不提供默认实现。
func buildSimpleParser(app *cherry.AppBuilder) cfacade.INetParser {
	agentActor := simple.NewActor("user")
	agentActor.AddConnector(cconnector.NewTCP(":10011"))
	agentActor.AddConnector(cconnector.NewWS(app.Address()))

	agentActor.SetOnNewAgent(func(newAgent *simple.Agent) {
		childActor := &ActorAgent{}
		//newAgent.AddOnClose(childActor.onSessionClose)
		agentActor.Child().Create(newAgent.SID(), childActor)
	})

	// 设置大头&小头
	agentActor.SetEndian(binary.LittleEndian)
	// 设置心跳时间
	agentActor.SetHeartbeatTime(60 * time.Second)
	// 设置积压消息数量
	agentActor.SetWriteBacklog(64)

	// 设置数据路由函数(未设置时使用 simple 解析器的默认路由)
	// agentActor.SetOnDataRoute(你的路由函数)

	// 设置消息节点路由(建议配合data-config组件进行使用)
	// mid = 1 的消息路由到  gate节点.user的Actor.login函数上
	agentActor.AddNodeRoute(1, &simple.NodeRoute{
		NodeType: "gate",
		ActorID:  "user",
		FuncName: "login",
	})

	return agentActor
}
