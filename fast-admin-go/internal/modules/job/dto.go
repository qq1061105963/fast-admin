package job

type SaveRequest struct {
	ID             string `json:"id"`
	JobName        string `json:"jobName" binding:"required,max=64"`
	JobGroup       string `json:"jobGroup"`
	BeanName       string `json:"beanName" binding:"required,max=128"`
	MethodName     string `json:"methodName"`
	MethodParams   string `json:"methodParams"`
	CronExpression string `json:"cronExpression" binding:"required"`
	MisfirePolicy  int8   `json:"misfirePolicy"`
	Concurrent     bool   `json:"concurrent"`
	Status         int8   `json:"status"`
	Remark         string `json:"remark"`
}
