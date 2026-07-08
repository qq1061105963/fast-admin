package role

import "time"

type Dto struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Status      int       `json:"status"`
	Remark      string    `json:"remark"`
	IsEnabled   bool      `json:"isEnabled"`
	DataScope   int       `json:"dataScope"`
	DeptIDs     []string  `json:"deptIds,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

func toDto(r Role) Dto {
	status := 0
	if r.IsEnabled {
		status = 1
	}
	return Dto{
		ID: r.ID, Name: r.Name, Code: r.Code, Status: status, Remark: r.Remark,
		IsEnabled: r.IsEnabled, DataScope: int(r.DataScope),
		CreateTime: r.CreatedAt, UpdateTime: r.UpdatedAt,
	}
}

type SaveRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name" binding:"required,max=100"`
	Status      int      `json:"status"`
	Remark      string   `json:"remark"`
	DataScope   int      `json:"dataScope"`
	DeptIDs     []string `json:"deptIds"`
	Permissions []string `json:"permissions"`
}

type SelectOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type PageQuery struct {
	ID        string `form:"id"`
	Name      string `form:"name"`
	Status    string `form:"status"`
	Remark    string `form:"remark"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Page      int    `form:"page,default=1"`
	Size      int    `form:"pageSize,default=10"`
}
