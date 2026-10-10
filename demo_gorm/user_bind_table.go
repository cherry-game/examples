package main

// UserBindTable 对应数据库表 user_bind(uid 绑定第三方平台帐号)。
//
// 这是 gorm 的"模型结构体"写法, 几个约定:
//
//   - 结构体名默认映射到表名(下划线风格), 也可以用 TableName() 显式指定。
//     本示例的表名是 user_bind, 与默认推导一致, 但显式写出来更清楚。
//   - 每个字段用 struct tag 描述它对应的列:
//     column       列名
//     primary_key  主键
//     comment      列注释(建表时用, 这里只是留档)
//   - 类型要和数据库对得上: bigint 用 int64, int 用 int32, varchar 用 string。
//
// 建表语句见同目录的 db.sql。
type UserBindTable struct {
	UID      int64  `gorm:"column:uid;primary_key;comment:'用户唯一id'" json:"uid"`
	SdkId    int32  `gorm:"column:sdk_id;comment:'sdk配置id'" json:"sdkId"`
	PID      int32  `gorm:"column:pid;comment:'平台id'" json:"pid"`
	OpenId   string `gorm:"column:open_id;comment:'平台帐号open_id'" json:"openId"`
	BindTime int64  `gorm:"column:bind_time;comment:'绑定时间'" json:"bindTime"`
}

// TableName 显式指定表名。
// 不实现这个方法时, gorm 会把结构体名转成下划线复数(这里会变成 user_bind_tables), 与库表不符。
func (*UserBindTable) TableName() string {
	return "user_bind"
}
