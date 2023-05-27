package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	assets "kinozaltv_monitor"
	"kinozaltv_monitor/api"
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
)

var globalConfig = config.GlobalConfig

func main() {
	wsChan := make(chan string)

	e := echo.New()
	e.HideBanner = true

	// Set allowed origins for CORS to allow requests from any origin
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	// Middleware
	e.Use(middleware.Logger())

	e.Use(middleware.Recover())

	// Set channel for adding torrent by url
	handler := api.NewApiHandler(qbittorrent.UrlChan)
	msgPool := api.NewMsgPool(wsChan)

	go qbittorrent.TorrentChecker(wsChan, qbittorrent.UrlChan)

	var contentHandler = echo.WrapHandler(http.FileServer(http.FS(assets.Assets)))
	var contentRewrite = middleware.Rewrite(map[string]string{"/*": "/frontend/$1"})

	e.GET("/*", contentHandler, contentRewrite)

	// API routes
	e.GET("/api/torrents", api.GetTorrentList)
	e.POST("/api/add", handler.AddTorrentUrl)
	e.DELETE("/api/remove", api.RemoveTorrentUrl)

	// Websocket route
	e.GET("/ws", msgPool.HandleWsConnections)
	// Run ws pool
	go msgPool.Start()
	e.Logger.Fatal(e.Start(":" + globalConfig.ListenPort))
}
