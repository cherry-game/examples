package main

import (
	"github.com/cherry-game/cherry"
	ccode "github.com/cherry-game/cherry/code"
	cgin "github.com/cherry-game/cherry/components/gin"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cconnector "github.com/cherry-game/cherry/net/connector"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cserializer "github.com/cherry-game/cherry/net/serializer"
)

// 启动main函数运行聊天室程序
func main() {
	// 配置cherry引擎
	app := cherry.Configure(
		"../../config/demo-chat.json", // path       profile配置的文件路径
		"room-1",                      // nodeId     节点id,每个节点都有唯一的节点id，并且他们归属于某一个节点类型(nodeType)
		true,                          // isFrontend 节点为前端类型，则可使用connector连接器组件
		cherry.Standalone,             // nodeMode   单节点模式
	)

	// 设置json做为数据序列化的方式，系统默认:protobuf
	// json			cserializer.NewJSON()
	// protobuf		cserializer.NewProtobuf()
	app.SetSerializer(cserializer.NewJSON())

	// 创建pomelo网络数据包解析器，它同时也是一个actor
	agentActor := pomelo.NewActor("user")
	// 添加websocket连接器, 根据业务需要可添加多类型的connector
	agentActor.AddConnector(cconnector.NewWS(":34590"))
	// 创建Agent时，关联onClose函数
	agentActor.SetOnNewAgent(func(newAgent *pomelo.Agent) {

		newAgent.AddOnClose(func(agent *pomelo.Agent) {
			session := agent.Session()
			if !session.IsBind() {
				return
			}

			// 发送玩家断开连接的消息给room actor
			req := &Int64{
				Value: session.Uid,
			}

			agentActor.Call(".room", "exit", req)
			clog.Debugf("[sid = %s,uid = %d] session disconnected.",
				session.Sid,
				session.Uid,
			)
		})
	})

	// 设置数据路由函数
	agentActor.SetOnDataRoute(onDataRoute)

	// 设置网络包解析器
	app.SetNetParser(agentActor)

	//添加actor
	app.AddActors(
		&actorRoom{},
	)

	// 启动http server
	httpServerComponent(app)

	// 运行cherry引擎
	app.Startup()
}

func onDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	session := pomelo.BuildSession(agent, msg)

	if msg.Route == "room.room.login" {
		targetPath := cfacade.NewChildPath(agent.NodeId(), route.HandleName(), session.Sid)
		pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
		return
	}

	// session未绑定uid，踢下线
	if !session.IsBind() {
		agent.ResponseCode(session, ccode.SessionUIDNotBind, true)
		return
	}

	targetPath := cfacade.NewPath(agent.NodeId(), route.HandleName())
	pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
}

// 为了省事，构造一个http server用于部署我们的客户端h5静态文件
func httpServerComponent(app *cherry.AppBuilder) {
	// 启动后访问 http://127.0.0.1:8081/ 即可
	httpComp := cgin.New("web", "127.0.0.1:8081")
	// http server使用gin组件搭建，这里增加一个RecoveryWithZap中间件
	httpComp.Use(cgin.RecoveryWithZap(true))

	// 直接映射h5客户端静态文件到根目录
	httpComp.Static("/", "../static/")
	// 把http server组件注册到cherry引擎中
	app.Register(httpComp)
}
