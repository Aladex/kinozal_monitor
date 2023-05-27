package api

import (
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"kinozaltv_monitor/database"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
)

var qbUser = qbittorrent.GlobalQbittorrentUser

var (
	upgrader = websocket.Upgrader{
		// Allow all origins
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type ApiHandler struct {
	urlChan chan string
}

type MsgHandler struct {
	msg chan string
}

func NewApiHandler(urlChan chan string) *ApiHandler {
	return &ApiHandler{
		urlChan: urlChan,
	}
}

type MsgPool struct {
	connections map[*websocket.Conn]bool
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	broadcast   chan []byte
	msg         chan string
}

func NewMsgPool(msgChan chan string) *MsgPool {
	return &MsgPool{
		broadcast:   make(chan []byte),
		register:    make(chan *websocket.Conn),
		unregister:  make(chan *websocket.Conn),
		connections: make(map[*websocket.Conn]bool),
		msg:         msgChan,
	}
}

func (pool *MsgPool) HandleWsConnections(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	pool.register <- ws

	for {
		msg := <-pool.msg
		// Send message to all connections
		c.Logger().Info("Sending message to all connections: ", msg)
		pool.SendToAll(msg)
		if err := ws.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			c.Logger().Error(err)
			return err
		}
	}
}

func (pool *MsgPool) SendToAll(msg string) {
	pool.broadcast <- []byte(msg)
}

func (pool *MsgPool) Start() {
	for {
		select {
		case connection := <-pool.register:
			pool.connections[connection] = true
		case connection := <-pool.unregister:
			if _, ok := pool.connections[connection]; ok {
				delete(pool.connections, connection)
			}
		case message := <-pool.broadcast:
			for connection := range pool.connections {
				if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
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
	// Get url from request body by POST method
	url := c.FormValue("url")
	// Check if url is empty
	if url == "" {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": "url is empty"})
	}
	// Send url to channel
	h.urlChan <- url

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
