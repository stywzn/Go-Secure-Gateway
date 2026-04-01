package main

import (
	"log"
	"net/http"
	"time"

	"Go-Secure-Gateway/internal/config"
	"Go-Secure-Gateway/internal/middleware"
	"Go-Secure-Gateway/internal/proxy"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

func main() {
	// 1. 加载配置
	cfg := config.LoadConfig("configs/config.yaml")

	r := gin.Default()

	// 2. 暴露 Prometheus 监控指标 (公开接口)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 3. 基础探针与调试接口 (公开接口)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/debug/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": 9527,
			"exp":     time.Now().Add(time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString([]byte(cfg.JWT.Secret))
		c.JSON(http.StatusOK, gin.H{"token": tokenString})
	})

	// 4. 初始化全局限流器 (2 qps, 突发 5)
	limiter := middleware.NewIPRateLimiter(rate.Limit(2), 5)

	// ==========================================
	// 5. 核心受保护路由组 (Gateway Core)
	// ==========================================
	// 把需要保护的路由全部放进 protectedGroup
	protectedGroup := r.Group("/")

	// 修复错误 1: 传入 limiter 参数
	protectedGroup.Use(middleware.RateLimitMiddleware(limiter))

	// 修复错误 2: 统一 JWT 鉴权 (使用配置里的 Secret)
	protectedGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))

	// 6. 动态挂载 YAML 配置中的微服务路由
	log.Println("========================================")
	log.Println("正在加载微服务路由表...")

	for _, route := range cfg.Routes {
		prefix := route.PathPrefic // 注意：这里保留了你 struct 里的拼写 PathPrefic
		target := route.TargetURL

		// 为每个后端目标初始化我们写的高性能原生代理引擎
		proxyEngine, err := proxy.NewProxyEngine(target)
		if err != nil {
			log.Fatalf("代理引擎初始化失败 [%s]: %v", target, err)
		}

		// 修复错误 3 & 4: 废弃 GinReverseProxy，改用 gin.WrapH 包装我们自己的原生引擎
		// 将其挂载到受保护的路由组中
		protectedGroup.Any(prefix, gin.WrapH(proxyEngine))
		protectedGroup.Any(prefix+"/*path", gin.WrapH(proxyEngine))

		log.Printf("映射成功: %-15s => %s", prefix, target)
	}
	log.Println("========================================")

	// 7. 启动服务
	log.Printf("Go-Secure-Gateway 启动，监听端口 %s", cfg.Server.Port)
	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("网关服务异常退出: %v", err)
	}
}
