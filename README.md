# cherry 示例仓库

本仓库是 [cherry](https://github.com/cherry-game/cherry) 游戏服务端框架的示例集合。
每个目录都是一个可以独立运行的完整程序，代码里带大量新手向注释，建议配合源码一起读。

## 环境要求

| 项 | 要求 |
| --- | --- |
| Go | **>= 1.24**（cherry v1.6.x 的 go.mod 要求 go 1.24.0） |
| nats | 只有 `demo_cluster` / `test_discovery` 需要，仓库自带 `3rd/nats-server/nats-server.exe` |
| MySQL | 只有 `demo_gorm` 需要 |
| Redis | 只有 `test_redis` 需要 |

没写过 Go 的话，先看[环境安装与配置](https://github.com/cherry-game/cherry/blob/master/_docs/env-setup.md)。

## 推荐学习路径

按这个顺序读，每一步只引入少量新概念：

1. **[demo_chat](demo_chat) —— 先跑起来**
   单节点聊天室，只有 3 个文件。跑通它就能看到 cherry 的全部骨架：
   应用配置 → 网络解析器 → actor → 组件。

2. **[demo_gorm](demo_gorm) —— 接数据库**
   在 demo_chat 的基础上加一个 gorm 组件和定时查库的 actor，
   理解"组件是什么、节点的私有配置怎么读、actor 定时器怎么用"。

3. **[demo_cluster](demo_cluster) —— 完整的多节点服务端**
   五个节点（master/center/web/gate/game）、集群发现、跨节点路由、顶号处理。
   这是本仓库最有价值也最难的部分，前面两个读透了再来。

4. **其余 `test_*` 目录** —— 零散的功能验证
   每个目录演示一个点，按需要挑着看即可。

## 示例一览

| 目录 | 说明 | 需要的外部服务 |
| --- | --- | --- |
| [demo_chat](demo_chat) | 单节点聊天室（websocket + json） | 无 |
| [demo_gorm](demo_gorm) | gorm 组件 + 定时查库 | MySQL |
| [demo_cluster](demo_cluster) | 五节点完整示例（gate/game/center/web/master） | nats、MySQL |
| test_actor | actor 与子 actor 的创建、调用 | 无 |
| test_data_config | 策划配表加载与热重载 | 无 |
| test_discovery | 节点发现服务（master + 两个 client） | nats |
| test_func_call | Go 反射调用函数 | 无 |
| test_gin | gin 组件与路由 | 无 |
| test_gob | gob 序列化 | 无 |
| test_goroutine | goroutine 与 context 退出 | 无 |
| test_logger | 日志组件 | 无 |
| test_nats | nats 的收发/应答 | nats |
| test_protobuf | protobuf 编解码 | 无 |
| test_redis | redis 读写与订阅 | Redis |
| test_zap | zap 日志切割 | 无 |

## 核心概念速查

读代码时遇到这些词，可以先回这里对一下：

| 概念 | 一句话解释 |
| --- | --- |
| **node（节点）** | 一个 cherry 进程。每个节点有唯一的 `nodeID`，并归属于某个 `nodeType` |
| **nodeType** | 节点类型，如 `gate`、`game`、`web`。消息按类型路由，"这条消息该谁处理"看的就是它 |
| **actor** | 最小并发单元。每个 actor 有独立 goroutine 和消息队列，**内部字段不需要加锁** |
| **actor 路径** | `节点id.actorID` 或 `节点id.actorID.子id`；点号前为空（`.room`）表示本节点 |
| **component（组件）** | 可插拔单元，有 `Init`/`OnAfterInit`/`OnStop` 生命周期。定时器、数据库、http 服务都是组件 |
| **Agent** | "一条客户端连接"在服务端的代表，由网络解析器创建，可以往回推消息、可以踢人 |
| **session** | **一条消息的快照**（不是长连接的状态）。带 sid/uid 和连接上暂存的键值对 |
| **route** | 消息路由字符串，如 `room.room.login`，点号分成 `节点类型.handler名.方法名` 三段 |
| **Local / Remote 邮箱** | `Local()` 收"按路由投递"的消息，`Remote()` 收"用 Call/CallWait 调用"的消息。**同节点的 Call 也走 Remote** |
| **discovery（发现服务）** | 谁在负责记录"集群里有哪些节点"。示例用的是 nats 模式，由 master 节点维护 |

## 快速开始

以最简单的 demo_chat 为例（不需要任何外部服务）：

```bash
git clone https://github.com/cherry-game/examples.git
cd examples/demo_chat/room
go run .
```

看到下面这行就说明启动成功了：

```
Websocket connector listening at Address :34590
```

然后用浏览器打开两个 <http://127.0.0.1:8081/>，在输入框里发消息，两个页面都能收到广播。

> 注意：demo_chat 和 demo_cluster 的 web 服务都监听 8081，**不要同时启动**。

## 常见问题

**启动时报 `net packet parser is nil.`**
节点被配置成前端节点（`isFrontend = true`），但没有提供网络包解析器。
如果这个节点本来就不接收客户端连接，把它改成 `false` 即可。

**节点一直打印 `center node connect fail. retrying in 2 seconds.`**
说明它连不上 center 节点。先确认 center 已经启动，再看 nats 是否在跑（`127.0.0.1:4222`）。

**`demo_cluster` 节点启动后互相发现不了**
本示例的发现服务依赖 nats。先启动 `3rd/nats-server/run_nats.bat`，
并且 **master 节点必须最先启动**。

**用 `go build -race` 时看到 `WARNING: DATA RACE`**
cherry v1.6.8 的框架内部有两处已知的数据竞争（`Application.Running()` 的非原子读、
`Member.LastAt` 的无锁写）。在 amd64 上功能不受影响，但会让 `-race` 构建报错。
这两处在框架仓库，不在本示例仓库内。
