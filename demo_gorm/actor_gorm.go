// ActorDB 演示"actor + 数据库"的常见写法:
// 在 OnInit 里取出数据库句柄并缓存到 actor 字段, 之后用定时器周期性地查库。
//
// 为什么把 DB 句柄存在 actor 里?
// 因为 actor 的字段只被它自己的 goroutine 访问, 不需要加锁;
// 而且 actor 上的消息和定时任务是串行的, 不会出现同一时刻并发查库的情况。
package main

import (
	"time"

	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	cherryGORM "github.com/cherry-game/components/gorm"
	"gorm.io/gorm"
)

// ActorDB 周期查询 user_bind 表的演示 actor。
type ActorDB struct {
	cactor.Base // 嵌入后即拥有 Child()/Timer()/Event() 等能力

	// centerDB 在 OnInit 里赋值, 之后只读, 所以并发访问是安全的。
	centerDB *gorm.DB
}

// AliasID 返回 actorID, 决定 actor 路径中的第二段: 本示例为 "game-1.db"。
func (p *ActorDB) AliasID() string {
	return "db"
}

// OnInit 在 actor 开始工作前调用一次, 适合做"初始化 + 注册定时任务"。
//
// 这个示例里涉及三处配置, 对应 config/demo-gorm.json 的三个地方:
//
//	app.Register(cherryGORM.NewComponent())  -> 告诉引擎"本节点要用 gorm 组件"(见 main.go)
//	db_id_list.center_db_id = "center_db_1"  -> 写在 game-1 的 __settings__ 里, 声明本节点用哪个库
//	db.center_db_group[0]                    -> 真正的连接串/账号/密码
func (p *ActorDB) OnInit() {
	// 获取gorm组件
	// 注意: Find() 找不到组件时返回 nil 接口, 此时单值类型断言 `.(*T)` 会直接 panic,
	// 后面的 nil 判断根本执行不到。这里用双值断言, 才能给出有意义的报错。
	// 变量名避开 gorm, 以免遮蔽 gorm.io/gorm 包名。
	gormComponent, ok := p.App().Find(cherryGORM.Name).(*cherryGORM.Component)
	if !ok || gormComponent == nil {
		clog.Panicf("[component = %s] not found.", cherryGORM.Name)
	}

	// Settings() 返回的是"本节点独有的配置", 也就是配置文件里该节点 __settings__ 的内容。
	// 这里读取 db_id_list.center_db_id, 拿到 "center_db_1" 这个逻辑库名。
	centerDbID := p.App().Settings().GetConfig("db_id_list").GetString("center_db_id")

	// 用逻辑库名去组件里换出一个 *gorm.DB。换不到说明配置对不上(比如节点没被授权连这个库)。
	p.centerDB = gormComponent.GetDb(centerDbID)
	if p.centerDB == nil {
		clog.Panic("center_db_1 not found")
	}

	// Timer() 是 actor 自带的定时器:
	//
	//	Add(d, fn)      每 d 执行一次 fn, 直到 actor 退出
	//	AddOnce(d, fn)  延迟 d 后执行一次 fn
	//
	// 回调都在本 actor 的 goroutine 上执行, 所以可以放心读写 actor 的字段。
	p.Timer().Add(5*time.Second, p.selectDB)

	// 启动 1 秒后做一次分页查询演示。用 AddOnce 而不是 Add, 是为了避免刷屏。
	p.Timer().AddOnce(1*time.Second, p.selectPagination)
}

// selectDB 取 user_bind 表的第一行, 每 5 秒执行一次。
func (p *ActorDB) selectDB() {
	// First 会按主键升序取一条。传结构体指针, gorm 会把结果填进去。
	userBindTable := &UserBindTable{}
	tx := p.centerDB.First(userBindTable)
	if tx.Error != nil {
		// 常见错误: 表不存在、账号密码错、库名不对。示例里只打日志, 不中断。
		clog.Warn(tx.Error)
	}

	clog.Infof("%+v", userBindTable)
}

// selectPagination 分页查询演示, 启动 1 秒后执行一次。
func (p *ActorDB) selectPagination() {
	list, count := p.pagination(1, 10)
	clog.Infof("count = %d", count)

	for _, table := range list {
		clog.Infof("%+v", table)
	}
}

// pagination 分页查询。
//
// 返回 (当前页数据, 总条数)。page 从 1 开始。
func (p *ActorDB) pagination(page, pageSize int) ([]*UserBindTable, int64) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	var list []*UserBindTable
	var count int64

	// Count 先查总数: SELECT count(*) FROM user_bind
	p.centerDB.Model(&UserBindTable{}).Count(&count)

	if count > 0 {
		list = make([]*UserBindTable, pageSize)

		// Limit/Offset 是分页的标准写法:
		//   LIMIT pageSize OFFSET (page-1)*pageSize
		s := p.centerDB.Limit(pageSize).Offset((page - 1) * pageSize)
		if err := s.Find(&list).Error; err != nil {
			clog.Warn(err)
		}
	}

	return list, count
}
