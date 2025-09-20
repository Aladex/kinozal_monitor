package main

import (
	assets "kinozaltv_monitor"
	"kinozaltv_monitor/api"
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/config"
	customMiddleware "kinozaltv_monitor/middleware"
	"kinozaltv_monitor/models"
	"kinozaltv_monitor/qbittorrent"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var globalConfig = config.GlobalConfig

func main() {
	// Initialize the tracker manager with all available trackers
	models.InitializeTrackers(globalConfig)

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
	e.Logger.Fatal(e.Start(":" + globalConfig.ListenPort))
}
