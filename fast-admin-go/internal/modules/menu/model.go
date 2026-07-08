package menu

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// MenuType 对应 SysMenuType：1菜单 2目录 3按钮 4嵌入式 5链接。
type MenuType int8

const (
	TypeMenu     MenuType = 1
	TypeCatalog  MenuType = 2
	TypeButton   MenuType = 3
	TypeEmbedded MenuType = 4
	TypeLink     MenuType = 5
)

var typeToString = map[MenuType]string{
	TypeMenu: "menu", TypeCatalog: "catalog", TypeButton: "button",
	TypeEmbedded: "embedded", TypeLink: "link",
}

var stringToType = map[string]MenuType{
	"MENU": TypeMenu, "CATALOG": TypeCatalog, "BUTTON": TypeButton,
	"EMBEDDED": TypeEmbedded, "LINK": TypeLink,
}

// Menu 对应 sys_menu 表。
type Menu struct {
	model.BaseModel
	PID                    string   `gorm:"column:pid" json:"pid"`
	Name                   string   `gorm:"column:name" json:"name"`
	Code                   string   `gorm:"column:code" json:"code"`
	Type                   MenuType `gorm:"column:type" json:"type"`
	Status                 int8     `gorm:"column:status" json:"status"`
	Path                   string   `gorm:"column:path" json:"path"`
	ActivePath             string   `gorm:"column:active_path" json:"activePath"`
	Component              string   `gorm:"column:component" json:"component"`
	Icon                   string   `gorm:"column:icon" json:"icon"`
	MetaActiveIcon         string   `gorm:"column:meta_active_icon" json:"metaActiveIcon"`
	MetaTitle              string   `gorm:"column:meta_title" json:"metaTitle"`
	MetaOrder              int      `gorm:"column:meta_order" json:"metaOrder"`
	MetaAffixTab           bool     `gorm:"column:meta_affix_tab" json:"metaAffixTab"`
	MetaKeepAlive          bool     `gorm:"column:meta_keep_alive" json:"metaKeepAlive"`
	MetaHideInMenu         bool     `gorm:"column:meta_hide_in_menu" json:"metaHideInMenu"`
	MetaHideChildrenInMenu bool     `gorm:"column:meta_hide_children_in_menu" json:"metaHideChildrenInMenu"`
	MetaHideInBreadcrumb   bool     `gorm:"column:meta_hide_in_breadcrumb" json:"metaHideInBreadcrumb"`
	MetaHideInTab          bool     `gorm:"column:meta_hide_in_tab" json:"metaHideInTab"`
	MetaBadge              string   `gorm:"column:meta_badge" json:"metaBadge"`
	MetaBadgeType          string   `gorm:"column:meta_badge_type" json:"metaBadgeType"`
	MetaBadgeVariants      string   `gorm:"column:meta_badge_variants" json:"metaBadgeVariants"`
	MetaIframeSrc          string   `gorm:"column:meta_iframe_src" json:"metaIframeSrc"`
	MetaLink               string   `gorm:"column:meta_link" json:"metaLink"`
	Remark                 string   `gorm:"column:remark" json:"remark"`
}

func (Menu) TableName() string { return "sys_menu" }
