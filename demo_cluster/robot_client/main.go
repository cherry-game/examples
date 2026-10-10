// 压测机器人: 用 Go 模拟大量客户端并发跑完整条登录链路。
//
// 它对理解整个 demo_cluster 很有用 —— 因为它把"一个真实客户端要做什么"按顺序写了一遍,
// 而且用的是 cherry 自带的 pomelo 客户端(net/parser/pomelo/client), 走 tcp/protobuf 协议:
//
//  1. HTTP 调 web 节点的 /register 批量注册开发账号
//  2. HTTP 调 /login 换取 token
//  3. TCP 连 gate 节点(:10011), 发送 gate.user.login 完成"用户登录"
//  4. 依次发送 game.player.select / create / enter 完成"角色登录"
//
// 全部跑完后进程不会退出, 会保持连接以便在 game 节点观察在线人数, 按 Ctrl+C 结束。
//
// 运行前需要先起好 master / center / web / gate / game 五个节点。
// 机器人数量由下面的 maxRobotNum 控制, 首次试跑建议先改成几十个。
package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	chttp "github.com/cherry-game/cherry/extend/http"
	clog "github.com/cherry-game/cherry/logger"
	pomeloClient "github.com/cherry-game/cherry/net/parser/pomelo/client"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	jsoniter "github.com/json-iterator/go"
)

var (
	maxRobotNum       = 5000                    // 运行x个机器人
	url               = "http://127.0.0.1:8081" // web node
	addr              = "127.0.0.1:10011"       // 网关地址(正式环境通过区服列表获取)
	serverId    int32 = 10001                   // 测试的游戏服id
	pid               = "2126001"               // 测试的sdk包id
	printLog          = false                   // 是否输出详细日志
)

func main() {
	// 准备账号名/密码对。示例里账号名和密码相同, 都是 test1/test2/...
	accounts := make(map[string]string)
	for i := 1; i <= maxRobotNum; i++ {
		key := fmt.Sprintf("test%d", i)
		accounts[key] = key
	}

	// 第 1 步: 到 web 节点批量注册开发账号(已存在的会返回"账号已存在", 不影响后续流程)
	RegisterDevAccount(url, accounts)

	// 每个机器人配对一个 Add/Done, 等全部进入游戏
	wg := sync.WaitGroup{}

	for userName, password := range accounts {
		wg.Add(1)

		go func(userName, password string) {
			defer wg.Done()
			RunRobot(url, pid, userName, password, addr, serverId, printLog)
		}(userName, password)

		time.Sleep(time.Duration(rand.Int31n(10)) * time.Millisecond)
	}

	wg.Wait()
	clog.Infof("all %d robots are in game. press Ctrl+C to exit.", len(accounts))

	// 机器人连接需要保留, 便于在 game 节点观察 onlineCount, 因此等退出信号而不是直接结束。
	// 原实现是 `wg.Add(1)` 之后从未调用 `wg.Done()` 的 WaitGroup,
	// 靠计数永不为 0 来达到"永久阻塞"的效果: 语义不清, 且无法通过 Ctrl+C 优雅退出。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}

func RegisterDevAccount(url string, accounts map[string]string) {
	requestURL := fmt.Sprintf("%s/register", url)

	for key, val := range accounts {
		params := map[string]string{
			"account":  key,
			"password": val,
		}

		jsonBytes, _, err := chttp.GET(requestURL, params)
		if err != nil {
			clog.Warn(err)
			return
		}

		rsp := &code.Result{}
		err = jsoniter.Unmarshal(jsonBytes, rsp)
		if err != nil {
			clog.Warn(err)
			return
		}

		clog.Debugf("register account = %s, result = %+v", key, rsp)
	}
}

func RunRobot(url, pid, userName, password, addr string, serverId int32, printLog bool) *Robot {

	// 创建客户端
	cli := New(
		pomeloClient.New(
			pomeloClient.WithRequestTimeout(10*time.Second),
			pomeloClient.WithErrorBreak(true),
		),
	)
	cli.PrintLog = printLog

	// 登录获取token
	if err := cli.GetToken(url, pid, userName, password); err != nil {
		clog.Error(err)
		return nil
	}

	// 根据地址连接网关
	if err := cli.ConnectToTCP(addr); err != nil {
		clog.Error(err)
		return nil
	}

	if cli.PrintLog {
		clog.Infof("tcp connect %s is ok", addr)
	}

	// 随机休眠
	cli.RandSleep()

	// 用户登录到游戏节点
	err := cli.UserLogin(serverId)
	if err != nil {
		clog.Warn(err)
		return nil
	}

	if cli.PrintLog {
		clog.Infof("user login is ok. [user = %s, serverId = %d]", userName, serverId)
	}

	//cli.RandSleep()

	// 查看是否有角色
	err = cli.PlayerSelect()
	if err != nil {
		clog.Warn(err)
		return nil
	}

	//cli.RandSleep()

	// 创建角色
	err = cli.ActorCreate()
	if err != nil {
		clog.Warn(err)
		return nil
	}

	//cli.RandSleep()

	// 角色进入游戏
	err = cli.ActorEnter()
	if err != nil {
		clog.Warn(err)
		return nil
	}

	elapsedTime := cli.StartTime.NowDiffMillisecond()
	clog.Debugf("[%s] is enter to game. elapsed time:%dms", cli.TagName, elapsedTime)

	//cli.Disconnect()

	return cli
}
