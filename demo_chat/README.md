# 单节点精简版聊天室示例

用 cherry 引擎构建一个简单的多人聊天室。

- 客户端是 h5 页面，用 pomelo 协议通过 **websocket** 连接，序列化方式为 **json**
- 服务端只有 3 个文件，是整个仓库里最适合入门的示例

## 你会学到

- `cherry.Configure(...)` 的四个参数分别是什么
- 网络解析器与 Agent（"一条连接"在服务端的代表）
- actor 的创建、注册消息处理函数，以及 `Local()` 和 `Remote()` 的区别
- 消息路由（route 字符串如何决定消息去向）
- 组件是什么（例子里的 http server）

## 要求

- GO版本 >= 1.24 (cherry v1.6.x 要求 go 1.24.0 以上)

不需要任何外部服务（不用 nats、不用数据库）。

未使用过`Golang`的开发者，请参考[环境安装与配置](https://github.com/cherry-game/cherry/blob/master/_docs/env-setup.md) 进行准备工作。

## 操作步骤

### 克隆

- git clone https://github.com/cherry-game/examples.git
- 或通过github下载源码的方式。点击`code`按钮`Download zip`文件

### 用 GoLand 开发调试 - 推荐

- 找到`room/main.go`文件，点击`debug`

### 用 Visual Studio Code 开发调试

- 在VSCode的左侧栏找到`运行和调试(Debug)`按钮,选择`demo-chat`，点击`绿色小三角`

> 注意工作目录：程序按相对路径读配置（`../../config/demo-chat.json`），
> 所以必须**在 `demo_chat/room` 目录下**启动。

### 测试

- 在`终端(terminal)`面板中看到 `Websocket connector listening at Address :34590` 代表启动成功
- 在浏览器打开两个页面(`http://127.0.0.1:8081`)，在文本框中输入聊天内容并点击`send`按钮，两个页面将会收到聊天内容的广播

> 两个端口的分工：**:34590** 是给客户端的 websocket，**:8081** 只是托管 h5 静态页面。
> `demo_cluster` 的 web 节点也用 8081，两者**不要同时启动**。

## 一条消息的完整旅程

以客户端发送聊天内容为例：

```
浏览器                gate(解析器Actor)              room(Actor)
  │                        │                             │
  │─ ws 连接 ─────────────▶│  建 Agent，并建一条以 sid 为名的子 actor
  │                        │
  │─ request room.room.login ─▶│
  │                        │─ 走 onDataRoute 路由 ──────▶│ login()
  │                        │                             │ 分配 uid、Bind 到 Agent
  │◀───────── response ────│◀────────────────────────────│
  │                        │
  │─ notify room.room.syncMessage ─▶│
  │                        │─ 走 onDataRoute 路由 ──────▶│ syncMessage()
  │◀── push onMessage ─────│◀──── broadcast 给所有人 ────│
```

## 协议一览

客户端与服务端之间只有这么几个消息，定义在 `room/protocol.go`：

| 方向 | 名字 | 类型 | 说明 |
| --- | --- | --- | --- |
| C → S | `room.room.login` | request | 登录，携带昵称 |
| S → C | （login 的响应） | response | `{code: 0}` 表示成功 |
| C → S | `room.room.syncMessage` | notify | 发一条聊天消息 |
| S → C | `onNewUser` | push | 有人进入房间（广播） |
| S → C | `onMessage` | push | 聊天内容（广播） |
| S → C | `onBalance` | push | 余额变化，只发给发消息的人 |

## 配置

- 涉及的环境配置文件在 `../config/demo-chat.json`

## 关于actor model的使用

- 从`room/main.go`文件可得知，节点启动时通过`pomelo.NewActor("user")`创建了一个`user actor`. 该`actor`用于管理客户端连接.
- 通过`app.AddActors(...)`可得知，注册了`room`actor，用于房间管理
- 如果需要创建多个聊天房间，可以通过room的子actor实现

## 源码导航

| 文件 | 先看什么 |
| --- | --- |
| `room/main.go` | 程序入口。从 `cherry.Configure` 一路读到底，注释里解释了每个概念 |
| `room/actor_room.go` | 业务逻辑。重点看 `OnInit` 里 `Local()` 与 `Remote()` 的注册 |
| `room/protocol.go` | 协议结构体，改这里就等于改客户端协议 |
| `static/index.html` | h5 客户端，可以对照着看协议是怎么用的 |

## 排查

**浏览器打开 8081 是空白 / 404**
静态文件路径是相对的，确认是从 `demo_chat/room` 目录启动的。

**连接被拒绝，或浏览器控制台报 websocket 错误**
确认终端里已经打印出 `Websocket connector listening at Address :34590`；
如果 34590 被占用，改 `main.go` 里 `cconnector.NewWS(":34590")` 的端口。
