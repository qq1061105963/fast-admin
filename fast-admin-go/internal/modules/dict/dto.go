package dict

// Status 用指针类型是为了区分"请求里没带这个字段"（新增时默认启用）和
// "显式传了 0"（禁用），plain int8 的零值会把这两种情况混为一谈。
type TypeSaveRequest struct {
	ID       string `json:"id"`
	DictName string `json:"dictName" binding:"required,max=128"`
	DictType string `json:"dictType" binding:"required,max=128"`
	Status   *int8  `json:"status"`
	Remark   string `json:"remark"`
}

type DataSaveRequest struct {
	ID        string `json:"id"`
	DictType  string `json:"dictType" binding:"required"`
	DictLabel string `json:"dictLabel" binding:"required,max=128"`
	DictValue string `json:"dictValue" binding:"required,max=128"`
	DictSort  int    `json:"dictSort"`
	CSSClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	IsDefault bool   `json:"isDefault"`
	Status    *int8  `json:"status"`
	Remark    string `json:"remark"`
}
