package api

import (
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/database"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
	"sync"
)

type QbittorrentUser interface {
	DeleteTorrent(hash string, deleteFiles bool) error
	GetDownloadPaths() ([]string, error)
}

// qbUser is a global variable for storing qbittorrent user data
var qbUser = qbittorrent.GlobalQbittorrentUser

var log = logger.New("api")

var (
	upgrader = websocket.Upgrader{
		// Allow all origins
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type ApiHandler struct {
	torrentData chan common.TorrentData
}

type MsgHandler struct {
	msg chan string
}

func NewApiHandler(torrentData chan common.TorrentData) *ApiHandler {
	return &ApiHandler{
		torrentData: torrentData,
	}
}

type MsgPool struct {
	connections map[*websocket.Conn]bool
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	broadcast   chan string
	connMux     sync.Mutex // Mutex to protect connections
}

func NewMsgPool(msgChan chan string) *MsgPool {
	return &MsgPool{
		broadcast:   msgChan,
		register:    make(chan *websocket.Conn),
		unregister:  make(chan *websocket.Conn),
		connections: make(map[*websocket.Conn]bool),
	}
}

func (pool *MsgPool) HandleWsConnections(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer func() {
		pool.unregister <- ws
		ws.Close()
	}()

	pool.register <- ws

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Error("Error during reading from connection: ", err.Error(), nil)
			break
		}
	}
	return nil
}

func (pool *MsgPool) Start() {
	for {
		select {
		case connection := <-pool.register:
			pool.connMux.Lock() // Lock when modifying the connections map
			pool.connections[connection] = true
			pool.connMux.Unlock() // Unlock after modifying the connections map
		case connection := <-pool.unregister:
			pool.connMux.Lock() // Lock when modifying the connections map
			if _, ok := pool.connections[connection]; ok {
				delete(pool.connections, connection)
			}
			pool.connMux.Unlock() // Unlock after modifying the connections map
		case message := <-pool.broadcast:
			for connection := range pool.connections {
				if err := connection.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
					log.Error("Error during sending message to connection: ", err.Error(), nil)
					pool.unregister <- connection
					connection.Close()
					break
				}
			}
		}
	}
}

// AddTorrentUrl is a function for adding a torrent by url
func (h *ApiHandler) AddTorrentUrl(c echo.Context) error {
	// Create new TorrentData instance
	var torrentData common.TorrentData

	// Bind request body to torrentData
	if err := c.Bind(&torrentData); err != nil {
		// If there's any error return 400 Bad Request
		return c.JSON(400, map[string]string{"error": "Bad Request"})
	}

	// Check if url is empty
	if torrentData.Url == "" {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": "url is empty"})
	}

	// Send url to channel
	h.torrentData <- common.TorrentData{Url: torrentData.Url, DownloadPath: torrentData.DownloadPath}

	return c.JSON(200, map[string]string{"status": "ok"})
}

// RemoveTorrentUrl is a function for removing a torrent by ID
func RemoveTorrentUrl(c echo.Context) error {
	// Read JSON from request body
	var json map[string]string
	err := c.Bind(&json)
	if err != nil {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	// Get torrent name from JSON
	torrentUrl := json["url"]
	torrentHash := json["hash"]
	// Delete torrent from qbittorrent by name
	err = qbUser.DeleteTorrent(torrentHash, true)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Delete torrent from database
	err = database.DeleteRecord(database.DB, torrentUrl)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, map[string]string{"status": "ok"})
}

// GetTorrentList is a function for getting a list of torrents
func GetTorrentList(c echo.Context) error {
	dbTorrents, err := database.GetAllRecords(database.DB)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Convert to JSON
	return c.JSON(200, dbTorrents)
}

// GetDownloadPaths is a function for getting a list of download paths from qbittorrent
func GetDownloadPaths(c echo.Context) error {
	paths, err := qbUser.GetDownloadPaths()
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Convert to JSON
	return c.JSON(200, paths)
}
