package main

import (
	"fgo-calc-backend/internal/config"
	"fgo-calc-backend/internal/handler"
	"fgo-calc-backend/internal/repository"
	"fgo-calc-backend/internal/service"
	"flag"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 加载配置
	configPath := flag.String("config", "config.dev.json", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化 Repository
	repo, err := repository.NewRepository(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// 3. 初始化 Service
	svc := service.NewCalculatorService(repo)

	// 4. 初始化 Handler
	h := handler.NewHandler(repo, svc, cfg)

	// 5. 设置 Gin 路由
	r := gin.Default()
	h.Register(r)

	// 6. 启动服务器
	log.Printf("Server starting on %s", cfg.Port)
	if err := r.Run(cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
