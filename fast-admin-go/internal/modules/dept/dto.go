package dept

import "time"

type TreeNode struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	PID        string     `json:"pid,omitempty"`
	PName      string     `json:"pname,omitempty"`
	Status     bool       `json:"status"`
	Remark     string     `json:"remark,omitempty"`
	IsEnabled  bool       `json:"isEnabled"`
	CreateTime time.Time  `json:"createTime"`
	Children   []TreeNode `json:"children,omitempty"`
}

type SaveRequest struct {
	ID        string `json:"id"`
	Name      string `json:"name" binding:"required,max=20"`
	PID       string `json:"pid"`
	Status    bool   `json:"status"`
	Remark    string `json:"remark"`
	IsEnabled bool   `json:"isEnabled"`
}

func toTreeNode(d Dept) TreeNode {
	return TreeNode{
		ID: d.ID, Name: d.Name, PID: d.PID, Status: d.Status,
		Remark: d.Remark, IsEnabled: d.IsEnabled, CreateTime: d.CreatedAt,
	}
}
