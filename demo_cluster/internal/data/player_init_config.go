package data

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/types"
)

type (
	PlayerInitRow struct {
		Gender int32           `json:"gender"` // 性别
		Level  int32           `json:"level"`  // 初始等级
		Items  types.I32I64Map `json:"items"`  // 初始的道具列表
		Heroes types.I32I64Map `json:"heroes"` // 初始的英雄列表
	}

	// 角色初始化数据
	playerInitConfig struct {
		maps map[int32]*PlayerInitRow
	}
)

func (p *playerInitConfig) Name() string {
	return "playerInitConfig"
}

func (p *playerInitConfig) Init() {
	p.maps = make(map[int32]*PlayerInitRow)
}

func (p *playerInitConfig) OnLoad(maps interface{}, _ bool) (int, error) {
	list, ok := maps.([]interface{})
	if !ok {
		return 0, cherryError.Error("maps convert to []interface{} error.")
	}

	loadMaps := make(map[int32]*PlayerInitRow)
	for index, data := range list {
		loadConfig := &PlayerInitRow{}
		err := DecodeData(data, loadConfig)
		if err != nil {
			cherryLogger.Warnf("decode error. [row = %d, %v], err = %s", index+1, loadConfig, err)
			continue
		}
		loadMaps[loadConfig.Gender] = loadConfig
	}

	p.maps = loadMaps

	return len(list), nil
}

func (p *playerInitConfig) OnAfterLoad(_ bool) {
}

func (p *playerInitConfig) Get(gender int32) (*PlayerInitRow, bool) {
	val, found := p.maps[gender]
	return val, found
}
