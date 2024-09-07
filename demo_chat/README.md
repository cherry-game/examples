# 单节点精简版聊天室示例

- 使用cherry引擎构建一个简单的多人聊天室程序
- 本示例为h5客户端，使用`pomelo-ws-client`做为客户端sdk，连接类型为`websocket`，序列化类型为`json`

## 要求

- GO版本 >= 1.18

未使用过`Golang`的开发者，请参考[环境安装与配置](https://github.com/cherry-game/cherry/blob/master/_docs/env-setup.md) 进行准备工作。

## 操作步骤

### 克隆

- git clone https://github.com/cherry-game/examples.git
- 或通过github下载源码的方式。点击`code`按钮`Download zip`文件

### 用 GoLand 开发调试 - 推荐

- 找到`room/main.go`文件，点击`debug`

### 用 Visual Studio Code 开发调试

- 在VSCode的左侧栏找到`运行和调试(Debug)`按钮,选择`demo-chat`，点击`绿色小三角`

### 测试

- 在`终端(terminal)`面板中看到 `Websocket connector listening at Address :34590` 代表启动成功
- 在浏览器打开两个页面(`http://127.0.0.1:8081`)，在文本框中输入聊天内容并点击`send`按钮，两个页面将会收到聊天内容的广播

### 配置

- 涉及的环境配置文件在 `/config/demo-chat.json`

### 关于actor model的使用

- 从`room/main.go`文件可得知，节点启动时通过`pomelo.NewActor("user")`创建了一个`user actor`. 该`actor`用于管理客户端连接.
- 通过`app.AddActors(...)`可得知，注册了`room`actor，用于房间管理
- 如果需要创建多个聊天房间，可以通过room的子actor实现