// web 节点: 对外的 http 服务, 也是 h5 客户端的宿主。
//
// 客户端接入游戏的完整链路:
//
//  1. 连 web 节点拿 token 和区服列表
//     GET /register       注册开发账号
//     GET /login          用 pid + 账号密码换一个 base64 token
//     GET /server/list/:pid  取区服列表(告诉客户端该连哪个 gate)
//  2. 拿着 token 连 gate 节点的 tcp/websocket, 发送 gate.user.login 完成"用户登录"
//  3. 之后走 game 节点的角色协议(查角色/建角色/进游戏)
//
// web 节点本身不保存任何连接, 可以横向扩展多个实例。
package web

import (
	"github.com/cherry-game/cherry"
	cherryFile "github.com/cherry-game/cherry/extend/file"
	cherryCron "github.com/cherry-game/components/cron"
	cherryGin "github.com/cherry-game/components/gin"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/web/controller"
	"github.com/cherry-game/examples/demo_cluster/nodes/web/sdk"
	"github.com/gin-gonic/gin"
)

// Run 启动 web 节点。
func Run(profileFilePath, nodeID string) {
	// 配置cherry引擎,加载profile配置文件
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)

	// 注册调度组件
	app.Register(cherryCron.New())

	// 注册检查中心服是否启动组件。
	// 注册/login 都要 rpc 调 center 拿账号和 uid, 所以必须等 center 就绪。
	app.Register(checkCenter.New())

	// 注册数据配表组件
	app.Register(data.New())

	// 加载http server组件。监听地址取自配置里本节点的 address(:8081)
	app.Register(httpServerComponent(app.Address()))

	// 加载sdk逻辑。
	// "sdk" 指第三方登录渠道, 示例里实现了两种:
	//	DevMode  开发模式, 直接用注册的开发账号登录
	//	QuickSDK 跳转第三方校验, 见 nodes/web/sdk/quick_sdk.go
	// 具体用哪个由账号所属 pid 对应的 sdk_id 决定。
	sdk.Init(app)

	// 启动cherry引擎
	app.Startup()
}

// httpServerComponent 组装并返回 gin 组件。
//
// 注意这里的静态文件路径是 "./static/"、模板路径是 "./view/",
// 都是相对"进程工作目录"的 —— 所以 web 节点必须在 demo_cluster/nodes 目录下启动。
func httpServerComponent(addr string) *cherryGin.Component {
	gin.SetMode(gin.DebugMode)

	// new http server
	httpServer := cherryGin.NewHttp("http_server", addr)
	httpServer.Use(cherryGin.Cors())

	// http server使用gin组件搭建，这里增加一个RecoveryWithZap中间件
	httpServer.Use(cherryGin.RecoveryWithZap(true))

	// 映射h5客户端静态文件到static目录，例如：http://127.0.0.1/static/protocol.js
	httpServer.Static("/static", "./static/")

	// 加载./view目录的html模板文件(详情查看gin文档)
	viewFiles := cherryFile.WalkFiles("./view/", ".html")
	if len(viewFiles) < 1 {
		panic("view files not found.")
	}
	httpServer.LoadHTMLFiles(viewFiles...)

	//注册 controller
	httpServer.Register(new(controller.Controller))

	return httpServer
}
