package config

type SaveRequest struct {
	ID          string `json:"id"`
	ConfigName  string `json:"configName" binding:"required,max=128"`
	ConfigKey   string `json:"configKey" binding:"required,max=128"`
	ConfigValue string `json:"configValue"`
	ConfigType  int8   `json:"configType"`
	Remark      string `json:"remark"`
}
