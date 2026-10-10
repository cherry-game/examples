// 房间 actor: 维护"房间里有哪些用户", 并负责把聊天内容广播给所有人。
//
// 关于 actor 的三个基础认知:
//
//  1. 每个 actor 有自己独立的 goroutine 和消息队列, 同一个 actor 上的消息是串行处理的。
//     所以 actor 内部的字段(userMap 等)不需要加锁 —— 这是 actor 模型最大的便利。
//     反过来说: 不要在这些函数里做阻塞操作(比如同步 HTTP), 否则会卡住整个 actor。
//
//  2. actor 用"路径"寻址, 路径形如:
//     两段式  "节点id.actorID"        例如 room-1.room
//     三段式  "节点id.actorID.子id"   例如 room-1.room.<sid>
//     路径里点号前为空(如 ".room")表示"本节点"。
//
//  3. actor 可以有子 actor(见 main.go 里按 sid 创建的连接子 actor),
//     子 actor 之间互相隔离, 适合"每个玩家/每个房间一个实例"这类场景。
package main

import (
	"fmt"
	"sync/atomic"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
)

var (
	// _nextUID 是本示例里最简单的"全局唯一 id"做法。
	// 注意它只在内存里递增, 进程重启后会从 1 重新开始 ——
	// 正式项目应该用数据库自增、redis INCR 或雪花算法(cherry 自带 extend/snowflake)。
	_nextUID int64
)

// newUID 返回下一个 uid。atomic 保证多个调用方拿到不重复的值。
func newUID() int64 {
	return atomic.AddInt64(&_nextUID, 1)
}

type (
	// actorRoom 房间 actor。
	//
	// 嵌入 cactor.Base 后, 就自动拥有了 Startup/Exit/Child()/Timer()/Event() 等能力。
	// 只需要按需重写 OnInit / OnStop 这些生命周期函数。
	actorRoom struct {
		cactor.Base

		// userMap 房间里的用户, key 是 uid。
		// 只被本 actor 的 goroutine 访问, 因此不需要加锁。
		userMap map[int64]*User // uid,user
	}

	// User 房间里一个用户的运行时数据。
	// 注意这些数据只在内存里, 进程重启就没了(见 demo_cluster 里的落库示例)。
	User struct {
		uid      cfacade.UID
		nickname string
		balance  int64 // 余额, 每发一条消息扣 1
		message  int   // 已发送的消息数
	}
)

// AliasID 返回该 actor 的 actorID。
// 它就是 actor 路径中的第二段: 这里返回 "room", 于是路径为 "room-1.room"。
func (p *actorRoom) AliasID() string {
	return "room"
}

// OnInit 在 actor 被创建后、开始收消息之前调用一次。
// 通常在这里完成两件事: 初始化字段、注册消息处理函数。
func (p *actorRoom) OnInit() {
	p.userMap = make(map[int64]*User)

	// Register(funcName, 处理函数) 把"函数名 -> 处理函数"登记到 actor 上,
	// 之后别人就可以按函数名发消息过来了。
	//
	// 这里有一个很容易踩的坑: Local() 和 Remote() 收的是两类不同来源的消息。
	//
	//	Local()   收的是"客户端消息被路由过来"的请求(走 PostLocal)
	//	Remote()  收的是"其他 actor 用 Call/CallWait 调用"的请求(走 PostRemote)
	//
	// 关键点: 即使是发给同一个节点的 Call, 走的也是 Remote。
	// 所以 main.go 里 `agentActor.Call(".room", "exit", req)` 注册的是 Remote 而不是 Local。
	// 两端对不上时不会报错, 只会"消息发出去了但没人处理", 排查时优先看这里。
	p.Local().Register("login", p.login)
	p.Local().Register("syncMessage", p.syncMessage)

	p.Remote().Register("exit", p.exit)
}

// OnLocalReceived 控制"收到 Local 消息时"的分发方式, 返回 (是否已处理, 是否不再继续分发)。
//
// 默认行为: 消息会被继续转发给它对应的子 actor(路径第三段就是子 actor 的 id)。
// 本示例返回 false, true 表示:
//   - 不需要框架回调, 由框架直接按注册的函数执行
//   - 不再往子 actor 转发, 全部在本 actor 内处理
//
// 因为聊天室的登录/发消息都由房间统一处理, 不需要每个连接一个子 actor。
func (*actorRoom) OnLocalReceived(_ *cfacade.Message) (bool, bool) {
	// 当接收local消息时，直接在当前actor执行(不再路由到子actor)
	return false, true
}

// login 处理客户端登录。
//
// 消息处理函数的签名是约定好的:
//
//	func (p *actorRoom) xxx(session *cproto.Session, req *SomeStruct)
//
// 第一个参数是会话上下文(框架自动注入, 包含 sid/uid 和连接上暂存的键值对),
// 第二个参数是客户端发来的数据体(框架按 json/protobuf 反序列化后注入)。
// 参数个数和类型可以按需组合, 也可以只要其中一个。
func (p *actorRoom) login(session *cproto.Session, req *LoginRequest) {
	clog.Debugf("nickname = %s", req.Nickname)

	// 已经绑定过 uid 说明这条连接登录过了, 直接忽略, 避免重复建号
	if session.IsBind() {
		return
	}

	// 按 sid 找到这条连接对应的 Agent。
	//
	// Agent 是"一条客户端连接"在服务端的代表, 由 pomelo 解析器管理,
	// 可以理解成"能往回推消息、能踢人下线"的那一端。
	agent, found := pomelo.GetAgentWithSID(session.Sid)
	if !found {
		return
	}

	// 给这条连接绑定一个 uid。绑定之后 session.IsBind() 就会返回 true,
	// 后续路由时也能按 uid 反查到这条连接。
	uid := newUID()
	agent.Bind(uid)

	user := &User{
		uid:      uid,
		nickname: req.Nickname,
		balance:  1000,
		message:  0,
	}

	p.userMap[uid] = user

	// 广播给房间内所有已登录用户(包括刚加入的这位)，通知有新用户进入房间
	newUserRequest := &NewUserBroadcast{
		Content: fmt.Sprintf("user join: %+v", req),
	}
	p.broadcast("onNewUser", newUserRequest)

	// Response 是"给这条请求回一个响应"。
	// 与 Push 的区别:
	//   Response  只发给发消息的这个客户端, 并且会带上请求的 mid, 客户端能对上号
	//   Push      主动推送给某个客户端, 不需要客户端先请求
	agent.Response(session, &LoginResponse{})
}

// exit 处理玩家断开连接。
//
// 注意这个函数没有 session 参数: 它是被 agentActor 用 Call 主动调用的(见 main.go),
// 调用方只带了 uid 这一个参数, 所以这里也只声明一个参数。
func (p *actorRoom) exit(req *Int64) {
	if req.Value < 1 {
		return
	}
	delete(p.userMap, req.Value)
}

// syncMessage 处理"发一条聊天消息".
//
// 这里能直接读到 session.Uid, 是因为前面 login 里已经 Bind 过;
// 如果客户端没登录就发这条消息, main.go 的 onDataRoute 会先把它拦掉。
func (p *actorRoom) syncMessage(session *cproto.Session, req *SyncMessage) {
	user, found := p.userMap[session.Uid]
	if !found {
		clog.Errorf("user not found: %v", session.Uid)
		return
	}

	user.message++
	user.balance--

	// 有新消息，广播给其他用户
	p.broadcast("onMessage", req)

	// 扣减当前用户的余额，并通知客户端。
	// 这条只发给发消息的人自己, 所以用 Push 而不是 broadcast。
	agent, found := pomelo.GetAgentWithSID(session.Sid)
	if found {
		agent.Push("onBalance", &UserBalanceResponse{
			CurrentBalance: user.balance,
		})
	}
}

// broadcast 把消息推给房间里的每一个用户。
//
// route 是推送给客户端的事件名, 客户端用 pomelo.on("事件名", 处理函数) 接收。
// 注意: 遍历的是 uid, 再按 uid 反查 Agent —— 之所以不直接存 Agent,
// 是因为连接可能断开重连, uid 到连接的映射由框架统一维护, 更可靠。
func (p *actorRoom) broadcast(route string, v interface{}) {
	for uid := range p.userMap {
		if agent, ok := pomelo.GetAgentWithUID(uid); ok {
			agent.Push(route, v)
		}
	}
}
