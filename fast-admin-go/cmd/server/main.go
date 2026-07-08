// Command server 是应用入口，对应 fast-application 里的 Spring Boot 启动类。
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/SirYuxuan/fast-admin-go/internal/bootstrap"
)

func main() {
	configDir := flag.String("config", "configs", "配置文件目录")
	env := flag.String("env", "", "运行环境（dev/test/prod），默认读 APP_ENV，未设置则为 dev")
	flag.Parse()

	app, err := bootstrap.New(*configDir, *env)
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
