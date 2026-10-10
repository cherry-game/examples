// ActorAgent: 每条客户端连接对应一个实例, 是"这条连接"在网关侧的全部状态。
//
// 创建时机见 gate.go 的 SetOnNewAgent:
//
//	agentActor.Child().Create(newAgent.SID(), childActor)   // actorID == sid
//
// 于是路径是 "<节点id>.user.<sid>"。之所以一连接一个 actor, 是因为:
//   - actor 内部字段天然串行访问, 不需要为每个连接加锁
//   - 玩家断线时, actor 退出即可自动清理它占用的资源
package gate

import (
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
	"github.com/cherry-game/examples/demo_cluster/internal/token"
)

var (
	// duplicateLoginCode "顶号"时发给旧连接的踢人包内容。
	// 它在 gate.Run() 里预先算好 —— 因为每个连接都会并发创建 ActorAgent,
	// 若在每个 ActorAgent.OnInit() 里赋值, 就是对同一变量的并发写(数据竞争)。
	duplicateLoginCode []byte
)

type (
	// ActorAgent 每个网络连接对应一个ActorAgent
	ActorAgent struct {
		cactor.Base
	}
)

// OnInit 注册本连接需要处理的消息。
func (p *ActorAgent) OnInit() {
	// duplicateLoginCode 已在 gate.Run() 中预先算好(见 gate.go),
	// 这里不再赋值: OnInit 会在每个连接创建时并发执行, 直接写包级变量会造成数据竞争。

	// Local: 客户端消息经路由后投递进来(route.go 里的 LocalDataRoute)
	p.Local().Register("login", p.login)

	// Remote: 被其他 actor 用 Call/CallWait 调用进来。
	// 注意即使是同节点的调用也走 Remote —— 所以别按"本地/远程"去理解,
	// 应该理解成"按路由投递"和"按函数调用"这两条不同的通道。
	p.Remote().Register("setSession", p.setSession)
}

// setSession 把 game 节点算出来的会话数据写回这条连接。
//
// 谁调用它? 玩家在 game 节点进入游戏后, game 侧会回调网关对应的 ActorAgent,
// 把 PlayerID 之类写进来。写进来的值会随之后的每条消息一起(作为 session 快照)带到 game 节点,
// route.go 里的 session.Contains(sessionKey.PlayerID) 判断的就是它。
//
// 参数里没有 session, 因为这是被 Call 调用的, 调用方只传了一个 pb.StringKeyValue。
func (p *ActorAgent) setSession(req *pb.StringKeyValue) {
	if req.Key == "" {
		return
	}

	// ActorID 就是 sid(创建时用的就是 sid), 所以按 sid 反查 Agent。
	// 第二个参数传 0 表示"这次只按 sid 找"; 传 uid 则是按 sid 优先、找不到再按 uid 找。
	if agent, ok := pomelo.GetAgent(p.ActorID(), 0); ok {
		agent.SetData(req.Key, req.Value)
	}
}

// login 用户登录，验证帐号 (*pb.LoginResponse, int32)
//
// 这是客户端连上网关后必须发的第一条协议(gate.user.login), 做四件事:
//
//  1. 校验 token 是否合法、是否过期
//  2. 校验 pid(渠道包 id)是否在配表里
//  3. 到 center 节点换取 uid(同一平台账号始终是同一个 uid)
//  4. 把 uid 绑到这条连接上, 并处理"顶号"
func (p *ActorAgent) login(session *cproto.Session, req *pb.LoginRequest) {
	agent, found := pomelo.GetAgent(p.ActorID(), 0)
	if !found {
		return
	}

	// 验证token
	userToken, errCode := p.validateToken(req.Token)
	if code.IsFail(errCode) {
		agent.Response(session, errCode)
		return
	}

	// 验证pid是否配置
	sdkRow := data.SdkConfig.Get(userToken.PID)
	if sdkRow == nil {
		agent.ResponseCode(session, code.PIDError, true)
		return
	}

	// 根据token带来的sdk参数，从中心节点获取uid。
	// 这是一次跨节点的 rpc 调用(CallWait), 会阻塞本 actor 直到 center 返回。
	uid, errCode := rpcCenter.GetUID(p.App(), sdkRow.SdkId, userToken.PID, userToken.OpenID)
	if uid == 0 || code.IsFail(errCode) {
		agent.ResponseCode(session, code.AccountBindFail, true)
		return
	}

	// 把 uid 绑定到这条连接。
	// 如果该 uid 之前已经在别的连接上登录过, 返回的 oldAgent 就是那条旧连接(顶号场景)。
	oldAgent, err := pomelo.Bind(session.Sid, uid)
	if err != nil {
		agent.ResponseCode(session, code.AccountBindFail, true)
		clog.Warn(err)
		return
	}

	// 挤掉之前的agent
	if oldAgent != nil {
		oldAgent.Kick(duplicateLoginCode, true)
	}

	// 通知其他网关节点, 把它们上面同一个 uid 的旧连接也踢掉
	p.checkGateSession(uid)

	// 把登录上下文写进这条连接, 之后每条消息的 session 快照都会带上它们。
	// route.go 里读的 ServerID 就是这里写进去的。
	agent.SetData(sessionKey.ServerID, cstring.ToString(req.ServerId))
	agent.SetData(sessionKey.PID, cstring.ToString(userToken.PID))
	agent.SetData(sessionKey.OpenID, userToken.OpenID)

	response := &pb.LoginResponse{
		Uid:    uid,
		Pid:    userToken.PID,
		OpenId: userToken.OpenID,
	}

	agent.Response(session, response)
}

// validateToken 校验客户端带来的 base64 token。
//
// 返回 (解析出的 token, 错误码)。错误码为 code.OK 表示通过。
func (p *ActorAgent) validateToken(base64Token string) (*token.Token, int32) {
	userToken, ok := token.DecodeToken(base64Token)
	if !ok {
		return nil, code.AccountTokenValidateFail
	}

	platformRow := data.SdkConfig.Get(userToken.PID)
	if platformRow == nil {
		return nil, code.PIDError
	}

	// 用配表里的 salt 重新算一次签名, 与 token 里带的比对
	statusCode, ok := token.Validate(userToken, platformRow.Salt)
	if !ok {
		return nil, statusCode
	}

	return userToken, code.OK
}

// checkGateSession 遍历"其他网关节点", 让它们把这个 uid 的旧连接踢掉。
//
// 多网关部署时, 同一个账号可能之前连的是 gate-2, 现在连到了 gate-1,
// 光在本地踢是不够的, 必须广播给所有同类节点。
func (p *ActorAgent) checkGateSession(uid cfacade.UID) {
	rsp := &cproto.PomeloKick{
		Uid:    uid,
		Reason: duplicateLoginCode,
	}

	// Discovery() 是发现服务, 拿到的就是 master 维护的节点列表。
	// 这里取"所有 gate 节点(除了自己)"。
	members := p.App().Discovery().ListByType(p.App().NodeType(), p.App().NodeID())
	for _, member := range members {
		// user是gate.go里自定义的agentActorID
		actorPath := cfacade.NewPath(member.GetNodeID(), "user")
		// 目标不是某个具体连接, 而是对端网关的解析器 actor —— 它内部再按 uid 找到连接
		p.Call(actorPath, pomelo.KickFuncName, rsp)
	}
}

// onSessionClose  当agent断开时，关闭对应的ActorAgent
//
// 在 gate.go 的 SetOnNewAgent 里通过 newAgent.AddOnClose 注册。
// 注意这里运行在连接的收尾路径上, 只做轻量的通知, 不要做耗时操作。
func (p *ActorAgent) onSessionClose(agent *pomelo.Agent) {
	// 没走过 login 的连接不会写 ServerID, 直接忽略
	serverId := agent.GetData(sessionKey.ServerID)
	if serverId == "" {
		return
	}

	// 通知game节点关闭session
	childId := cstring.ToString(agent.UID())
	if childId != "" {
		// 目标是 game 节点上的那个玩家子 actor: "game节点id.player.玩家uid"
		targetPath := cfacade.NewChildPath(serverId, "player", childId)
		p.Call(targetPath, "sessionClose", nil)
	}

	// 自己退出。actor 退出后, 它占用的资源会被框架回收。
	p.Exit()
	clog.Infof("sessionClose path = %s", p.Path())
}
