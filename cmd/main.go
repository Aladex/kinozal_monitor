package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	assets "kinozaltv_monitor"
	"kinozaltv_monitor/api"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
)

func main() {

	e := echo.New()
	e.HideBanner = true
	// Set CORS to allow all origins
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))
	// Middleware
	e.Use(middleware.Logger())

	e.Use(middleware.Recover())

	// Run gorooutine for checking torrents in the database and on the tracker
	go qbittorrent.TorrentChecker()

	var contentHandler = echo.WrapHandler(http.FileServer(http.FS(assets.Assets)))
	var contentRewrite = middleware.Rewrite(map[string]string{"/*": "/frontend/$1"})

	e.GET("/*", contentHandler, contentRewrite)

	// API routes
	e.GET("/api/torrents", api.GetTorrentList)
	e.POST("/api/add", api.AddTorrentUrl)
	e.DELETE("/api/remove", api.RemoveTorrentUrl)

	e.Logger.Fatal(e.Start(":1323"))
}
