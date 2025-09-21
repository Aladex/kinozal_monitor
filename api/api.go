package api

import (
	"encoding/json"
	"kinozaltv_monitor/common"
	"kinozaltv_monitor/database"
	logger "kinozaltv_monitor/logging"
	"kinozaltv_monitor/qbittorrent"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type QbittorrentUser interface {
	DeleteTorrent(hash string, deleteFiles bool) error
	GetDownloadPaths() ([]string, error)
}

// getQbUser returns the qbittorrent user instance
func getQbUser() QbittorrentUser {
	if qbittorrent.GlobalManager == nil {
		panic("qbittorrent GlobalManager not initialized")
	}
	return qbittorrent.GlobalManager.User
}

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
		if closeErr := ws.Close(); closeErr != nil {
			log.Error("Error closing websocket connection: ", closeErr.Error(), nil)
		}
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

			log.Info("websocket_client_connected", "New WebSocket client connected", map[string]string{
				"total_connections": strconv.Itoa(len(pool.connections)),
			})

			// Send current state to the new connection
			currentState := map[string]interface{}{
				"type": "current_state",
				"data": GetCheckInfos(),
			}
			jsonMsg, err := json.Marshal(currentState)
			if err != nil {
				log.Error("Error marshaling current state: ", err.Error(), nil)
				pool.unregister <- connection
				continue
			}

			log.Info("sending_current_state", "Sending current state to new client", map[string]string{
				"message_size":   strconv.Itoa(len(jsonMsg)),
				"torrents_count": strconv.Itoa(len(GetCheckInfos())),
			})

			if err := connection.WriteMessage(websocket.TextMessage, jsonMsg); err != nil {
				log.Error("Error sending current state to new connection: ", err.Error(), nil)
				pool.unregister <- connection
			} else {
				log.Info("current_state_sent", "Current state successfully sent to new client", nil)
			}
		case connection := <-pool.unregister:
			pool.connMux.Lock() // Lock when modifying the connections map
			if _, exists := pool.connections[connection]; exists {
				delete(pool.connections, connection)
				log.Info("websocket_client_disconnected", "WebSocket client disconnected", map[string]string{
					"remaining_connections": strconv.Itoa(len(pool.connections)),
				})
			}
			pool.connMux.Unlock() // Unlock after modifying the connections map
		case message := <-pool.broadcast:
			pool.connMux.Lock()
			activeConnections := len(pool.connections)
			pool.connMux.Unlock()

			if activeConnections > 0 {
				log.Info("broadcasting_message", "Broadcasting message to clients", map[string]string{
					"active_connections": strconv.Itoa(activeConnections),
					"message_preview": func() string {
						if len(message) > 100 {
							return message[:100] + "..."
						}
						return message
					}(),
				})
			}

			for connection := range pool.connections {
				if err := connection.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
					log.Error("Error during sending message to connection: ", err.Error(), nil)
					pool.unregister <- connection
					if closeErr := connection.Close(); closeErr != nil {
						log.Error("Error closing connection after write failure: ", closeErr.Error(), nil)
					}
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
	var jsonTorrent map[string]string
	err := c.Bind(&jsonTorrent)
	if err != nil {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	// Get torrent name from JSON
	torrentUrl := jsonTorrent["url"]
	torrentHash := jsonTorrent["hash"]
	// Delete torrent from qbittorrent by name
	err = getQbUser().DeleteTorrent(torrentHash, true)
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
	paths, err := getQbUser().GetDownloadPaths()
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	// Convert to JSON
	return c.JSON(200, paths)
}

// WatchTorrents is a function for set a watch flag for torrents
func (h *ApiHandler) WatchTorrent(c echo.Context) error {
	// Read JSON from request body
	var jsonTorrent map[string]string
	err := c.Bind(&jsonTorrent)
	if err != nil {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	// Get torrent name from JSON
	torrentUrl := jsonTorrent["url"]
	// Get watch period from JSON in minutes
	watchPeriod := jsonTorrent["watchPeriod"]
	// Convert watch period to int
	watchPeriodInt, err := strconv.Atoi(watchPeriod)
	if err != nil {
		// Return 400 Bad Request
		return c.JSON(400, map[string]string{"error": err.Error()})
	}

	// Set watch flag for torrent
	err = database.SetWatchFlag(database.DB, torrentUrl, watchPeriodInt)
	if err != nil {
		// Return 500 Internal Server Error
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, map[string]string{"status": "ok"})
}

// GetCheckInfos returns the current check information for all torrents
func GetCheckInfos() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	// Get all torrents from database to ensure we have complete information
	dbTorrents, err := database.GetAllRecords(database.DB)
	if err != nil {
		log.Error("get_db_records_for_check_infos", "Error getting database records", map[string]string{"error": err.Error()})
		return result
	}

	// For each torrent in database, provide check info (either existing or default)
	for _, dbTorrent := range dbTorrents {
		if info, exists := qbittorrent.TorrentCheckInfos[dbTorrent.Url]; exists {
			// Use existing check info
			result[dbTorrent.Url] = map[string]interface{}{
				"last_check_time":    info.LastCheckTime.Format(time.RFC3339),
				"last_check_success": info.LastCheckSuccess,
			}
		} else {
			// Provide default values for torrents without check info yet
			defaultTime := time.Now()
			result[dbTorrent.Url] = map[string]interface{}{
				"last_check_time":    defaultTime.Format(time.RFC3339),
				"last_check_success": true, // Assume success initially
			}

			// Initialize the check info in the global map for future use
			qbittorrent.TorrentCheckInfos[dbTorrent.Url] = &qbittorrent.TorrentCheckInfo{
				LastCheckTime:    defaultTime,
				LastCheckSuccess: true,
			}
		}
	}

	return result
}
