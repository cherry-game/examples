package ops

import (
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

var (
	pingReturn = &pb.Bool{Value: true}
)

type (
	ActorOps struct {
		cactor.Base
	}
)

func (p *ActorOps) AliasID() string {
	return "ops"
}

// OnInit 注册remote函数
func (p *ActorOps) OnInit() {
	p.Remote().Register("ping", p.ping)
}

// ping 请求center是否响应
func (p *ActorOps) ping() (*pb.Bool, int32) {
	return pingReturn, code.OK
}
