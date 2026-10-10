package checkCenter

import (
	"sync"
	"time"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryLogger "github.com/cherry-game/cherry/logger"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
)

// Component 启动时,检查center节点是否存活
//
// 注意: cherry v1.6.x 起, discovery 的 worker 节点要等应用进入 Running 状态后
// 才会向 master 注册, 而 Running 是在所有组件的 OnAfterInit() 返回之后才置位的。
// 若像旧版本那样在 OnAfterInit() 里阻塞等待 center, 会与 discovery 注册互相等待:
//
//	等待 center 出现 -> 需要 discovery 已注册 -> 需要 OnAfterInit 返回 -> 正卡在等待里
//
// 因此这里改为后台协程轮询, 让节点先正常完成启动与注册, 再确认 center 是否就绪。
type Component struct {
	cherryFacade.Component

	stop     chan struct{} // 关闭信号, 由 OnStop 关闭
	stopOnce sync.Once
}

func New() *Component {
	return &Component{
		stop: make(chan struct{}),
	}
}

func (c *Component) Name() string {
	return "run_check_component"
}

func (c *Component) OnAfterInit() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			if rpcCenter.Ping(c.App()) {
				cherryLogger.Info("center node is ready.")
				return
			}

			cherryLogger.Warn("center node connect fail. retrying in 2 seconds.")

			// 用 select 等待下一次重试, 应用停止时能立即退出而不是再等满一个周期
			select {
			case <-c.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (c *Component) OnStop() {
	c.stopOnce.Do(func() {
		close(c.stop)
	})
}
