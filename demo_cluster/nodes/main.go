// 多节点示例的统一入口。
//
// 五个节点类型都从这一个可执行文件启动, 靠第一个命令行参数区分:
//
//	master  发现服务, 记录"集群里有哪些节点", 不处理业务
//	center  中心服, 处理全局唯一的事务(账号、uid 分配等)
//	web     http 服务, 对外提供注册/登录/区服列表等接口, 也是 h5 客户端的宿主
//	gate    网关, 面向客户端, 负责连接管理、消息路由与转发
//	game    游戏逻辑服, 玩家的实际业务逻辑都在这里
//
// 启动顺序有讲究: master 必须最先起, center 次之, 其余节点可以并发起。
// gate / game / web 都注册了 check_center 组件, 会等 center 就绪后才继续启动。
//
// 本机调试时的常用命令(在 demo_cluster/nodes 目录下执行):
//
//	go run . master --path=../../config/demo-cluster.json --node=gc-master
//	go run . center --path=../../config/demo-cluster.json --node=gc-center
//	go run . web    --path=../../config/demo-cluster.json --node=gc-web-1
//	go run . gate   --path=../../config/demo-cluster.json --node=gc-gate-1
//	go run . game   --path=../../config/demo-cluster.json --node=10001
//
// 也可以直接用仓库里的 .vscode/launch.json, 已经为五个节点配置好了。
package main

import (
	"fmt"
	"os"

	cherryConst "github.com/cherry-game/cherry/const"
	"github.com/cherry-game/examples/demo_cluster/nodes/center"
	"github.com/cherry-game/examples/demo_cluster/nodes/game"
	"github.com/cherry-game/examples/demo_cluster/nodes/gate"
	"github.com/cherry-game/examples/demo_cluster/nodes/master"
	"github.com/cherry-game/examples/demo_cluster/nodes/web"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "game cluster node",
		Description: "game cluster node examples",
		Commands: []*cli.Command{
			versionCommand(),
			masterCommand(),
			centerCommand(),
			webCommand(),
			gateCommand(),
			gameCommand(),
		},
	}

	// 用 urfave/cli 解析命令行。每个子命令对应一种节点, 具体逻辑在各自的 Run() 里。
	_ = app.Run(os.Args)
}

// versionCommand 打印当前使用的 cherry 框架版本, 用于确认依赖版本是否正确。
func versionCommand() *cli.Command {
	return &cli.Command{
		Name:      "version",
		Aliases:   []string{"ver", "v"},
		Usage:     "view version",
		UsageText: "game cluster node version",
		Action: func(c *cli.Context) error {
			fmt.Println(cherryConst.Version())
			return nil
		},
	}
}

// masterCommand 启动发现服务节点。
// 它维护全集群的节点列表, 其他节点启动时向它注册, 并定期发心跳。
func masterCommand() *cli.Command {
	return &cli.Command{
		Name:      "master",
		Usage:     "run master node",
		UsageText: "node master --path=../../config/demo-cluster.json --node=gc-master",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			master.Run(path, node)
			return nil
		},
	}
}

// centerCommand 启动中心服节点。
// 处理全局唯一的业务: 开发账号注册、uid 分配等。多节点部署时只能有一个。
func centerCommand() *cli.Command {
	return &cli.Command{
		Name:      "center",
		Usage:     "run center node",
		UsageText: "node center --path=../../config/demo-cluster.json --node=gc-center",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			center.Run(path, node)
			return nil
		},
	}
}

// webCommand 启动 web 节点。
// 对外提供 http 接口(注册/登录/区服列表), 同时托管 h5 客户端页面。
func webCommand() *cli.Command {
	return &cli.Command{
		Name:      "web",
		Usage:     "run web node",
		UsageText: "node web --path=../../config/demo-cluster.json --node=gc-web-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			web.Run(path, node)
			return nil
		},
	}
}

// gateCommand 启动网关节点。
// 客户端连的是它; 它负责校验登录态, 并把业务消息转发给 game 节点。
func gateCommand() *cli.Command {
	return &cli.Command{
		Name:      "gate",
		Usage:     "run gate node",
		UsageText: "node gate --path=../../config/demo-cluster.json --node=gc-gate-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			gate.Run(path, node)
			return nil
		},
	}
}

// gameCommand 启动游戏逻辑节点。
// 注意它的 node 参数必须是纯数字(示例里是 10001), 因为启动时会用它做雪花算法的节点号。
func gameCommand() *cli.Command {
	return &cli.Command{
		Name:      "game",
		Usage:     "run game node",
		UsageText: "node game --path=../../config/demo-cluster.json --node=10001",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			game.Run(path, node)
			return nil
		},
	}
}

// getParameters 取出 --path 与 --node 两个参数。
func getParameters(c *cli.Context) (path, node string) {
	path = c.String("path")
	node = c.String("node")
	return path, node
}

// getFlag 定义所有节点共用的两个命令行参数。
//
//	--path  profile 配置文件路径(默认值按"在 nodes 目录下执行"来算)
//	--node  本节点的 nodeID, 必须能在配置文件的 node 段里找到同名条目
func getFlag() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "path",
			Usage:    "profile config path",
			Required: false,
			Value:    "../../config/demo-cluster.json",
		},
		&cli.StringFlag{
			Name:     "node",
			Usage:    "node id name",
			Required: true,
			Value:    "",
		},
	}
}
