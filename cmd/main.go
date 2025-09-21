package main

import (
	assets "kinozaltv_monitor"
	"kinozaltv_monitor/api"
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/config"
	logger "kinozaltv_monitor/logging"
	customMiddleware "kinozaltv_monitor/middleware"
	"kinozaltv_monitor/models"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var globalConfig = config.GlobalConfig

func main() {
	// Initialize the tracker manager with all available trackers
	models.InitializeTrackers(globalConfig)

	// Initialize qBittorrent manager
	err := qbittorrent.InitializeManager(globalConfig.QBUsername, globalConfig.QBPassword)
	if err != nil {
		panic("Failed to initialize qBittorrent manager: " + err.Error())
	}

	wsChan := make(chan string)
	urlChan := make(chan common.TorrentData, 100)

	e := echo.New()
	e.HideBanner = true

	// Set allowed origins for CORS to allow requests from any origin
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	// Middleware
	e.Use(customMiddleware.HTTPLogger())

	e.Use(middleware.Recover())

	// Set channel for adding torrent by url
	handler := api.NewApiHandler(urlChan)
	msgPool := api.NewMsgPool(wsChan)

	go qbittorrent.TorrentChecker()
	go qbittorrent.WsMessageHandler(wsChan, urlChan)

	var contentHandler = echo.WrapHandler(http.FileServer(http.FS(assets.Assets)))
	var contentRewrite = middleware.Rewrite(map[string]string{"/*": "/frontend/$1"})

	e.GET("/*", contentHandler, contentRewrite)

	// API routes
	e.GET("/api/torrents", api.GetTorrentList)
	e.GET("/api/download-paths", api.GetDownloadPaths)
	e.POST("/api/add", handler.AddTorrentUrl)
	e.POST("/api/watch", handler.WatchTorrent)

	e.DELETE("/api/remove", api.RemoveTorrentUrl)

	// Websocket route
	e.GET("/ws", msgPool.HandleWsConnections)
	// Run ws pool
	go msgPool.Start()

	// Initialize our custom logger
	log := logger.New("http_server")

	// Create HTTP server manually to avoid Echo's automatic logging
	serverAddr := ":" + globalConfig.ListenPort
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Log server start with our custom logger
	log.Info("server_start", "HTTP server started", map[string]string{
		"address": serverAddr,
		"port":    globalConfig.ListenPort,
	})

	// Start server and log fatal errors with our logger
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server_error", "HTTP server failed to start", map[string]string{
			"error":   err.Error(),
			"address": serverAddr,
		})
		panic("HTTP server failed to start: " + err.Error())
	}
}
