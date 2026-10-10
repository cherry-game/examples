// 客户端与服务端之间传输的数据结构(俗称"协议"或"消息体")。
//
// 约定:
//
//   - 每个请求/响应/推送各用一个 struct 表示, 字段名与客户端 JS 里的对象字段一一对应。
//   - struct tag(反引号里的 `json:"nickname"`)决定了序列化后的字段名。
//     本示例用 json 序列化, 所以 tag 写 json; 如果用 protobuf, 则由 .proto 文件生成结构体。
//   - 客户端发来的数据会被框架自动反序列化成对应 struct, 直接作为消息处理函数的第二个参数;
//     反过来, 传给 Response/Push 的 struct 也会被自动序列化后发给客户端。
//
// 所以: 改这里就等于改客户端协议, 两边必须同步修改。
package main

import "time"

// LoginRequest 登录请求
//
// 客户端调用示例(见 ../static/index.html):
//
//	pomelo.request("room.room.login", {"nickname": "guest123"}, callback)
type LoginRequest struct {
	Nickname string `json:"nickname"`
}

// LoginResponse 登录响应
//
// Code 为 0 表示成功, 非 0 见 cherry/code 包里的状态码定义。
type LoginResponse struct {
	Code int `json:"code"`
}

// NewUserBroadcast 新用户进入房间的广播内容。
//
// 服务端通过 Push("onNewUser", ...) 推给客户端,
// 客户端用 pomelo.on("onNewUser", ...) 接收。
type NewUserBroadcast struct {
	Content string `json:"content"`
}

// SyncMessage 同步聊天消息。
//
// 客户端是"通知(notify)"而不是"请求(request)", 因此没有响应。
// 服务端收到后会广播给房间里的所有人。
type SyncMessage struct {
	Name       string `json:"name"`
	Content    string `json:"content"`
	PacketTime int64  `json:"packetTime"`
}

// UserBalanceResponse 用户聊天次数响应。
//
// 服务端发完一条消息后, 只把最新余额推给发消息的人自己, 事件名为 "onBalance"。
type UserBalanceResponse struct {
	CurrentBalance int64 `json:"currentBalance"`
}

// Int64 一个只带一个 int64 字段的包装类型。
//
// 为什么需要它? 因为消息处理函数统一以"结构体"作为参数,
// 直接传裸的 int64 无法和"多个参数的调用"区分开。
// 这里用它承载"玩家断开连接"事件里的 uid。
type Int64 struct {
	Value int64 `json:"value"`
}

// PacketSpendTime 从打包到当前时间，花费了多少时间(纳秒)。
// 可以用它粗略衡量一条消息从客户端发出到服务端处理的耗时。
func (p *SyncMessage) PacketSpendTime() int64 {
	return time.Now().UnixNano() - p.PacketTime
}
