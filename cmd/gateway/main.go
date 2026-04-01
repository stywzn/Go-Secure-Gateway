package main

import (
	"Go-Secure-Gateway/internal/config"
	"Go-Secure-Gateway/internal/middleware"
	"Go-Secure-Gateway/internal/proxy"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

func main() {

	cfg := config.LoadConfig("configs/config.yaml")

	r := gin.Default()

	limiter := middleware.NewIPRateLimiter(rate.Limit(2), 5)
	// 把限流中间件挂载到全局
	r.Use(middleware.RateLimitMiddleware(limiter))

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Standard Kubernetes probes
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/readyz", func(c *gin.Context) { c.String(200, "ok") })

	targetURL := "http://your-backend-service:8080"

	proxyEngine, err := proxy.NewProxyEngine(targetURL)
	if err != nil {
		log.Fatalf("Failed to initialize proxy engine: %v", err)
	}

	// ==========================================
	// Gateway Core Routing Group
	// ==========================================
	// Apply your existing security and rate-limiting middlewares
	apiGroup := r.Group("/api")
	apiGroup.Use(middleware.RateLimit()) // Your existing token bucket implementation
	apiGroup.Use(middleware.JWTAuth())   // Your existing JWT validation

	{
		// CRITICAL: The handover point.
		// We use gin.WrapH to convert the standard net/http.Handler (our ProxyEngine)
		// into a Gin HandlerFunc.
		// Traffic goes: Gin Route -> Middlewares -> Native Proxy -> Backend
		apiGroup.Any("/*path", gin.WrapH(proxyEngine))
	}

	// 鉴权白名单
	publicPaths := map[string]bool{
		"/healthz":     true,
		"/readyz":      true,
		"/debug/token": true,
	}

	r.Use(func(c *gin.Context) {
		if publicPaths[c.Request.URL.Path] {
			c.Next()
			return
		}
		middleware.JWTAuth(cfg.JWT.Secret)(c)
	})

	// 基础探针与工具接口
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/debug/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": 9527,
			"exp":     time.Now().Add(time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString([]byte(cfg.JWT.Secret))
		c.JSON(http.StatusOK, gin.H{"token": tokenString})
	})

	//动态多服务路由
	log.Println("========================================")
	log.Println("正在加载微服务路由表...")
	// 遍历 yaml 路由数组 动态挂载到Gin上
	for _, route := range cfg.Routes {
		prefix := route.PathPrefic
		target := route.TargetURL

		r.Any(prefix, proxy.GinReverseProxy(target))
		r.Any(prefix+"/*path", proxy.GinReverseProxy(target))
		log.Printf("映射成功: %-15s => %s", prefix, target)
	}
	log.Println("========================================")

	log.Printf("Go-Secure-Gateway (启动. 监听端口 %s", cfg.Server.Port)
	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("网关服务异常退出: %v", err)
	}
}
