package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 1. Load configuration.
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// ready reflects whether the gateway should receive traffic. It flips to
	// false as soon as shutdown begins so Kubernetes drains this pod.
	var ready atomic.Bool

	// 2. Public operational endpoints (no auth).
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		if !ready.Load() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 3. Debug helper to mint a token — only when explicitly enabled. This is
	// a token-minting backdoor and must never be reachable in production.
	if cfg.Debug {
		logger.Warn("debug mode enabled: /debug/token is exposed — do NOT use in production")
		r.GET("/debug/token", func(c *gin.Context) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id": 9527,
				"exp":     time.Now().Add(time.Hour).Unix(),
			})
			tokenString, _ := token.SignedString([]byte(cfg.JWT.Secret))
			c.JSON(http.StatusOK, gin.H{"token": tokenString})
		})
	}

	// 4. Per-IP rate limiter.
	limiter := middleware.NewIPRateLimiter(rate.Limit(cfg.RateLimit.RPS), cfg.RateLimit.Burst)
	defer limiter.Stop()

	// 5. Protected route group: rate limit + JWT auth.
	protectedGroup := r.Group("/")
	protectedGroup.Use(middleware.RateLimitMiddleware(limiter))
	protectedGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))

	// 6. Mount dynamic routes from config.
	logger.Info("loading microservice route table")
	for _, route := range cfg.Routes {
		proxyEngine, err := proxy.NewProxyEngine(route, time.Duration(cfg.Server.UpstreamTimeoutSeconds)*time.Second, logger)
		if err != nil {
			logger.Error("failed to init proxy engine", "route", route.PathPrefix, "err", err)
			os.Exit(1)
		}

		protectedGroup.Any(route.PathPrefix, gin.WrapH(proxyEngine))
		protectedGroup.Any(route.PathPrefix+"/*path", gin.WrapH(proxyEngine))

		logger.Info("route mapped",
			"prefix", route.PathPrefix,
			"backends", route.Backends(),
			"strip_prefix", route.StripPrefix,
		)
	}

	// 7. Build the HTTP server with sane timeouts (mitigates Slowloris).
	srv := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           r,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}

	// 8. Start serving in the background.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("Go-Secure-Gateway starting", "addr", cfg.Server.Port)
		ready.Store(true)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// 9. Wait for a shutdown signal or a fatal server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("server error", "err", err)
		os.Exit(1)
	case sig := <-quit:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	// 10. Graceful shutdown: stop accepting traffic, then drain in-flight
	// requests within a bounded window.
	ready.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("gateway stopped cleanly")
}
