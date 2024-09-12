package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	chttp "github.com/cherry-game/cherry/extend/http"
	ctime "github.com/cherry-game/cherry/extend/time"
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
	wg := sync.WaitGroup{}
	wg.Add(1)

	accounts := make(map[string]string)
	for i := 1; i <= maxRobotNum; i++ {
		key := fmt.Sprintf("test%d", i)
		accounts[key] = key
	}

	RegisterDevAccount(url, accounts)

	for userName, password := range accounts {
		time.Sleep(time.Duration(rand.Int31n(10)) * time.Millisecond)
		go RunRobot(url, pid, userName, password, addr, serverId, printLog)
	}

	wg.Wait()
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

	elapsedTime := cli.StartTime.DiffInMillisecond(ctime.Now())
	clog.Debugf("[%s] is enter to game. elapsed time:%dms", cli.TagName, elapsedTime)

	//cli.Disconnect()

	return cli
}
