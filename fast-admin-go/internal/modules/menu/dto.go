package menu

// MenuMeta 是返回给前端的树节点里的 meta 字段。
type MenuMeta struct {
	Icon               string `json:"icon,omitempty"`
	ActiveIcon         string `json:"activeIcon,omitempty"`
	ActivePath         string `json:"activePath,omitempty"`
	Title              string `json:"title,omitempty"`
	Order              int    `json:"order"`
	AffixTab           bool   `json:"affixTab"`
	KeepAlive          bool   `json:"keepAlive"`
	HideInMenu         bool   `json:"hideInMenu"`
	HideChildrenInMenu bool   `json:"hideChildrenInMenu"`
	HideInBreadcrumb   bool   `json:"hideInBreadcrumb"`
	HideInTab          bool   `json:"hideInTab"`
	Badge              string `json:"badge,omitempty"`
	BadgeType          string `json:"badgeType,omitempty"`
	BadgeVariants      string `json:"badgeVariants,omitempty"`
	IframeSrc          string `json:"iframeSrc,omitempty"`
	Link               string `json:"link,omitempty"`
}

// TreeNode 是返回给前端的菜单树节点。
type TreeNode struct {
	ID        string     `json:"id"`
	PID       string     `json:"pid,omitempty"`
	Name      string     `json:"name"`
	AuthCode  string     `json:"authCode,omitempty"`
	Status    int8       `json:"status"`
	Type      string     `json:"type"`
	Path      string     `json:"path,omitempty"`
	Component string     `json:"component,omitempty"`
	Meta      MenuMeta   `json:"meta"`
	Children  []TreeNode `json:"children,omitempty"`
}

// SaveRequest 是新增/编辑菜单的请求体。
type SaveRequest struct {
	ID         string          `json:"id"`
	PID        string          `json:"pid"`
	Name       string          `json:"name" binding:"required,max=100"`
	AuthCode   string          `json:"authCode"`
	Status     int             `json:"status"`
	Type       string          `json:"type" binding:"required"`
	Path       string          `json:"path"`
	ActivePath string          `json:"activePath"`
	Component  string          `json:"component"`
	Remark     string          `json:"remark"`
	Meta       MenuMetaRequest `json:"meta"`
}

type MenuMetaRequest struct {
	Title              string `json:"title"`
	Icon               string `json:"icon"`
	ActiveIcon         string `json:"activeIcon"`
	Order              int    `json:"order"`
	AffixTab           bool   `json:"affixTab"`
	KeepAlive          bool   `json:"keepAlive"`
	HideInMenu         bool   `json:"hideInMenu"`
	HideChildrenInMenu bool   `json:"hideChildrenInMenu"`
	HideInBreadcrumb   bool   `json:"hideInBreadcrumb"`
	HideInTab          bool   `json:"hideInTab"`
	Badge              string `json:"badge"`
	BadgeType          string `json:"badgeType"`
	BadgeVariants      string `json:"badgeVariants"`
	IframeSrc          string `json:"iframeSrc"`
	Link               string `json:"link"`
}

func toTreeNode(m Menu) TreeNode {
	return TreeNode{
		ID:        m.ID,
		PID:       m.PID,
		Name:      m.Name,
		AuthCode:  m.Code,
		Status:    m.Status,
		Type:      typeToString[m.Type],
		Path:      m.Path,
		Component: m.Component,
		Meta: MenuMeta{
			Icon:               m.Icon,
			ActiveIcon:         m.MetaActiveIcon,
			ActivePath:         m.ActivePath,
			Title:              m.MetaTitle,
			Order:              m.MetaOrder,
			AffixTab:           m.MetaAffixTab,
			KeepAlive:          m.MetaKeepAlive,
			HideInMenu:         m.MetaHideInMenu,
			HideChildrenInMenu: m.MetaHideChildrenInMenu,
			HideInBreadcrumb:   m.MetaHideInBreadcrumb,
			HideInTab:          m.MetaHideInTab,
			Badge:              m.MetaBadge,
			BadgeType:          m.MetaBadgeType,
			BadgeVariants:      m.MetaBadgeVariants,
			IframeSrc:          m.MetaIframeSrc,
			Link:               m.MetaLink,
		},
	}
}
