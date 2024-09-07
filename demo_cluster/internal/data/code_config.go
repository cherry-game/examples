package data

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
)

type (
	codeRow struct {
		Code    int32  `json:"code"`    //提示代码
		Message string `json:"message"` //消息内容
	}

	// 状态码列表
	codeConfig struct {
		maps map[int32]*codeRow
	}
)

func (p *codeConfig) Name() string {
	return "codeConfig"
}

func (p *codeConfig) Init() {
	p.maps = make(map[int32]*codeRow)
}

func (p *codeConfig) OnLoad(maps interface{}, _ bool) (int, error) {
	list, ok := maps.([]interface{})
	if !ok {
		return 0, cherryError.Error("maps convert to []interface{} error.")
	}

	codeMaps := make(map[int32]string)

	loadMaps := make(map[int32]*codeRow)
	for index, data := range list {
		loadConfig := &codeRow{}
		err := DecodeData(data, loadConfig)
		if err != nil {
			cherryLogger.Warnf("decode error. [row = %d, %v], err = %s", index+1, loadConfig, err)
			continue
		}

		codeMaps[loadConfig.Code] = loadConfig.Message
		loadMaps[loadConfig.Code] = loadConfig
	}

	p.maps = loadMaps

	if len(codeMaps) > 0 {
		// TODO 把配置文件中状态码全部添加到code码中，这样方便获取
		code.AddAll(codeMaps)
	}

	return len(list), nil
}

func (p *codeConfig) OnAfterLoad(_ bool) {
}

func (p *codeConfig) Get(c int32) *codeRow {
	val, found := p.maps[c]
	if !found {
		return nil
	}
	return val
}

func (p *codeConfig) GetMessage(c int32) string {
	if val, found := p.maps[c]; found {
		return val.Message
	}
	return ""
}
