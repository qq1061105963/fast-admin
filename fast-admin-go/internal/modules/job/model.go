package job

import "github.com/SirYuxuan/fast-admin-go/internal/framework/model"

// Job 对应 sys_job。CronExpression 用 6 段 Quartz 风格（含秒），调度器用
// robfig/cron 的 WithSeconds() 模式解析，字段顺序一致，但 Quartz 的 misfire
// 语义/day-of-month 与 day-of-week 的 "?" 通配符不完全等价，属已知差异点。
type Job struct {
	model.BaseModel
	JobName        string `gorm:"column:job_name" json:"jobName"`
	JobGroup       string `gorm:"column:job_group" json:"jobGroup"`
	BeanName       string `gorm:"column:bean_name" json:"beanName"`
	MethodName     string `gorm:"column:method_name" json:"methodName"`
	MethodParams   string `gorm:"column:method_params" json:"methodParams"`
	CronExpression string `gorm:"column:cron_expression" json:"cronExpression"`
	MisfirePolicy  int8   `gorm:"column:misfire_policy" json:"misfirePolicy"`
	Concurrent     bool   `gorm:"column:concurrent" json:"concurrent"`
	Status         int8   `gorm:"column:status" json:"status"` // 1正常 0暂停
	Remark         string `gorm:"column:remark" json:"remark"`
}

func (Job) TableName() string { return "sys_job" }

// JobLog 对应 sys_job_log：纯日志表，物理删除。Status 三态：2执行中(先插入占位)
// / 1成功 / 0失败，执行完成后回写。
type JobLog struct {
	model.LogModel
	JobID        string `gorm:"column:job_id" json:"jobId"`
	JobName      string `gorm:"column:job_name" json:"jobName"`
	JobGroup     string `gorm:"column:job_group" json:"jobGroup"`
	BeanName     string `gorm:"column:bean_name" json:"beanName"`
	MethodName   string `gorm:"column:method_name" json:"methodName"`
	MethodParams string `gorm:"column:method_params" json:"methodParams"`
	Status       int8   `gorm:"column:status" json:"status"`
	CostTime     int64  `gorm:"column:cost_time" json:"costTime"`
	ErrorMsg     string `gorm:"column:error_msg" json:"errorMsg"`
}

func (JobLog) TableName() string { return "sys_job_log" }
