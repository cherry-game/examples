# gorm 组件演示

演示 cherry 如何接入 MySQL，以及 **actor + 定时任务** 的常见写法。

这个示例不接收客户端连接，启动后会每隔 5 秒查一次库并打印结果，用来验证数据库链路是否通畅。

## 涉及的知识点

- 注册并使用**组件**：`app.Register(cherryGORM.NewComponent())`
- 通过 `App().Find(组件名)` 取出组件
- 读取**节点私有配置**：`App().Settings()` 对应配置文件里该节点的 `__settings__`
- actor 的定时器：`Timer().Add(d, fn)` 循环执行、`Timer().AddOnce(d, fn)` 只执行一次

## 运行前准备

1. 准备一个 MySQL，并导入建表脚本：

   ```bash
   mysql -uroot -p < db.sql
   ```

   脚本会创建 `dev_center` 库和 `user_bind` 表，并插入 2 条测试数据。

2. 创建示例使用的账号（或改用你自己的账号）：

   ```sql
   CREATE USER IF NOT EXISTS 'dev_user'@'%' IDENTIFIED BY 'dev_password';
   GRANT ALL PRIVILEGES ON dev_center.* TO 'dev_user'@'%';
   FLUSH PRIVILEGES;
   ```

3. 确认 `../config/demo-gorm.json` 里的连接信息正确：

   ```json
   "db": {
     "center_db_group": [
       {
         "enable": true,
         "db_id": "center_db_1",
         "db_name": "dev_center",
         "host": "127.0.0.1",
         "user_name": "dev_user",
         "password": "dev_password"
       }
     ]
   }
   ```

   > 注意 `db` 分组里开关字段叫 **`enable`**（不是 `enabled`，那是 `node` 段用的）。
   > 两者拼写不同，是因为由不同的代码读取，写错不会报错只会静默使用默认值。

## 运行

```bash
cd demo_gorm
go run .
```

也可以用 VSCode 的调试配置 `demo-gorm`。

## 期望输出

```
[dbGroup =center_db_group, dbName = center_db_1] is connected.
count = 2
&{UID:1 SdkId:1 PID:1 OpenId:1 BindTime:0}
&{UID:2 SdkId:2 PID:2 OpenId:2 BindTime:0}
&{UID:1 SdkId:1 PID:1 OpenId:1 BindTime:0}      <- 每 5 秒重复一次
```

- `is connected`：gorm 组件按配置连上了库
- `count = 2`：分页查询拿到的总条数，与 `db.sql` 里的测试数据一致
- 最后一行：`Timer().Add(5*time.Second, ...)` 的循环查询

## 配置是怎么串起来的

三处配置，缺一不可：

| 位置 | 内容 | 作用 |
| --- | --- | --- |
| `main.go` | `app.Register(cherryGORM.NewComponent())` | 告诉引擎"本节点要用 gorm 组件" |
| `demo-gorm.json` 的 `game-1.__settings__` | `db_id_list.center_db_id = "center_db_1"` | 声明本节点可以连哪个库 |
| `demo-gorm.json` 的 `db.center_db_group` | 真正的连接串/账号/密码 | 实际建连接用的信息 |

## 排查

**`center_db_1 not found`**
`__settings__.db_id_list.center_db_id` 的值，在 `db` 段里找不到同名的 `db_id`。

**`Error 1045: Access denied`**
账号密码不对，或被授权的库与 `db_name` 不一致。

**`Table 'dev_center.user_bind' doesn't exist`**
`db.sql` 没导入，或导入到了别的库。

## 文件说明

| 文件 | 内容 |
| --- | --- |
| `main.go` | 程序入口：注册组件、注册 actor、启动 |
| `actor_gorm.go` | 业务 actor：取 DB 句柄、挂定时器、执行查询 |
| `user_bind_table.go` | gorm 的模型结构体，对应 `user_bind` 表 |
| `db.sql` | 建库建表脚本 |
