// 这是一个单节点多人聊天室示例, 也是 cherry 引擎最精简的入门程序。
// 建议按下面的顺序读这个文件:
//
//  1. cherry.Configure(...)  创建应用, 先弄清 nodeID / nodeType / isFrontend / nodeMode 四个概念
//  2. 注册 pomelo 网络解析器, 理解 "一条客户端连接 = 一个 Agent(它同时也是一个 actor)"
//  3. 注册业务 actor(room), 理解 actor 路径, 以及 Local 消息与 Remote 消息的区别
//  4. app.Startup() 之后引擎做了什么
//
// 配套的 h5 客户端在 ../static/index.html, 通过 pomelo 协议的 websocket 连到 :34590。
package main

import (
	"github.com/cherry-game/cherry"
	ccode "github.com/cherry-game/cherry/code"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cconnector "github.com/cherry-game/cherry/net/connector"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cserializer "github.com/cherry-game/cherry/net/serializer"
	cgin "github.com/cherry-game/components/gin"
)

// main 启动聊天室服务。
//
// 启动后可以观察到两个监听端口:
//   - :34590  websocket, 客户端(游戏/聊天)连这里
//   - :8081   http,      用来托管 h5 静态页面, 浏览器打开 http://127.0.0.1:8081/ 即可
func main() {
	// Configure 会读取 profile 配置文件并创建应用。四个参数的含义:
	//
	//	path        profile 配置文件的路径(相对当前进程的工作目录)
	//	nodeID      本节点的唯一 id
	//	isFrontend  是否前端节点。前端节点才允许挂 connector(网络连接器)接收客户端连接;
	//	            纯逻辑节点传 false
	//	nodeMode    运行模式。Standalone(单进程单节点) 或 Cluster(多进程多节点, 需要 nats)
	//
	// profile 文件里还通过 nodeID 反查本节点属于哪个 nodeType(见下面 pomelo.NewActor 的说明)。
	app := cherry.Configure(
		"../../config/demo-chat.json", // path       profile配置的文件路径
		"room-1",                      // nodeID     节点id,每个节点都有唯一的节点id，并且他们归属于某一个节点类型(nodeType)
		true,                          // isFrontend 节点为前端类型，则可使用connector连接器组件
		cherry.Standalone,             // nodeMode   单节点模式
	)

	// 设置数据序列化的方式。系统默认是 protobuf;
	// 这个示例为了配合网页端调试方便, 改用 json。
	//
	//	json      cserializer.NewJSON()
	//	protobuf  cserializer.NewProtobuf()
	app.SetSerializer(cserializer.NewJSON())

	// 创建 pomelo 网络数据包解析器。
	//
	// 这里的 "user" 是节点类型(nodeType)之外的另一个名字, 它同时决定了两件事:
	//   - 它是这个解析器 actor 的 actorID
	//   - 每条客户端连接都会以 "user" 为父, 按 sid 创建一个子 actor(见 SetOnNewAgent)
	//
	// 换句话说: 客户端连接的管理者本身也是一个 actor, 由引擎托管。
	agentActor := pomelo.NewActor("user")

	// 添加 websocket 连接器, 监听 :34590。
	// 根据业务需要可以同时添加多个不同类型的连接器, 例如:
	//
	//	agentActor.AddConnector(cconnector.NewTCP(":10011"))
	agentActor.AddConnector(cconnector.NewWS(":34590"))

	// 每当有一条新的客户端连接建立, 引擎就会调用这个回调。
	// 回调里的 newAgent 代表 "这一条连接", 它的生命周期与连接一致。
	agentActor.SetOnNewAgent(func(newAgent *pomelo.Agent) {

		// 注册连接关闭时的回调。
		// 注意: 断线时连接已经要销毁了, 所以这里只做"通知业务层"这种轻量动作,
		// 不要在回调里阻塞或做重活。
		newAgent.AddOnClose(func(agent *pomelo.Agent) {
			// 没绑定过 uid 的连接说明还没登录过, 不需要通知房间
			if !agent.IsBind() {
				return
			}

			// 发送玩家断开连接的消息给room actor
			req := &Int64{
				Value: agent.UID(),
			}

			// Call 是"发消息给某个 actor 的某个函数", 参数含义:
			//
			//	source  消息来源的 actor 路径(这里用解析器自己)
			//	target  目标 actor 路径, ".room" 里的点号前是空的,
			//	        表示"本节点上的 room actor"(见文件末尾对 actor 路径的说明)
			//	funcName 目标 actor 上注册的函数名, 对应 actorRoom.OnInit 里的 Register("exit", ...)
			//
			// Call 是异步的, 不会等结果; 需要等结果请用 CallWait。
			agentActor.Call(".room", "exit", req)

			clog.Debugf("[sid = %s,uid = %d] session disconnected.",
				agent.SID(),
				agent.UID(),
			)
		})
	})

	// 设置数据路由函数。
	// 客户端发来的每一条业务消息都会先经过这个函数, 由它决定"这条消息交给谁处理"。
	// 不设置的话引擎会用默认的 DefaultDataRoute。
	agentActor.SetOnDataRoute(onDataRoute)

	// 把解析器交给引擎。引擎在启动时会据此创建连接、监听端口。
	app.SetNetParser(agentActor)

	// 注册业务 actor。
	//
	// actor 是 cherry 里的最小并发单元: 每个 actor 有独立的 goroutine 和消息队列,
	// 同一个 actor 上的消息是串行处理的, 因此 actor 内部的字段不需要加锁。
	// 这里的 room actor 负责维护"房间里的用户列表", 定义见 actor_room.go。
	app.AddActors(
		&actorRoom{},
	)

	// 启动http server
	httpServerComponent(app)

	// 启动引擎。
	// 这一步会依次: 初始化各组件 -> 组件 OnAfterInit -> 加载网络解析器并开始监听 -> 阻塞主 goroutine,
	// 直到收到退出信号。
	app.Startup()
}

// onDataRoute 数据路由: 决定客户端发来的消息交给哪个 actor 的哪个函数处理。
//
// pomelo 的 route 字符串形如 "room.room.login", 由点号分成三段:
//
//	第一段 nodeType   期望由哪种节点处理(room / gate / game ...)
//	第二段 handleName handler 名字, 决定把消息投给哪个 actor
//	第三段 method     消息里的方法名(实际分发时框架按 msg.Route 再决定具体函数)
//
// 消息最终会被投递到 actor 路径 "节点id.handler名字" 或 "节点id.handler名字.sid" 上。
func onDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	// 为这条消息构造一个 session(会话上下文)。
	//
	// 关键概念: session 不是"一条长连接的状态", 而是"这一条消息的快照"。
	// cherry v1.6.x 起, 每个消息都会新建一个 session, 里面带着这条连接的 sid/uid 和
	// 连接上暂存的键值对(agent.SetData 写进去的那些)。
	// 这样做的原因是: 连接的读写发生在多个 goroutine 上, 每消息一份快照可以避免并发读写同一份数据。
	session := agent.NewSession(msg.ID)

	// 登录消息是唯一允许"还没绑定 uid"就发送的消息, 所以要单独处理。
	// 其余消息都必须先登录, 否则拒绝。
	if msg.Route == "room.room.login" {
		// 登录消息投给"以 sid 为子 actor 的那一个实例"。
		// 这里用 NewChildPath 生成三段式路径 "节点id.room.sid",
		// 这样一个房间 actor 下的每个连接都有独立的子 actor, 互不干扰。
		targetPath := cfacade.NewChildPath(agent.NodeID(), route.HandleName(), session.Sid)
		// cherry v1.6.8 起 LocalDataRoute 不再接收 session 参数, 由内部按 msg 构建
		pomelo.LocalDataRoute(agent, route, msg, targetPath)
		return
	}

	// session未绑定uid，踢下线
	if !session.IsBind() {
		agent.ResponseCode(session, ccode.SessionUIDNotBind, true)
		return
	}

	// 其余业务消息投给房间 actor 本身(两段式路径 "节点id.room"),
	// 由房间统一处理, 比如记账、广播。
	targetPath := cfacade.NewPath(agent.NodeID(), route.HandleName())
	pomelo.LocalDataRoute(agent, route, msg, targetPath)
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
	//
	// "组件(component)" 是引擎的可插拔单元, 有 Init / OnAfterInit / OnStop 等生命周期回调,
	// 可以按名字 App().Find("组件名") 取到。定时器、数据库、http 服务在 cherry 里都是组件。
	app.Register(httpComp)
}
