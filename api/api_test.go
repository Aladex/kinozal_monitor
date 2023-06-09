package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"kinozaltv_monitor/api"
)

func TestAddTorrentUrl(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("url=https://mytorrent.com/torrentfile.torrent"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	urlChan := make(chan string, 1)

	h := api.NewApiHandler(urlChan)

	if assert.NoError(t, h.AddTorrentUrl(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, `{"status":"ok"}`, rec.Body.String())
		t.Log("urlChan:", <-urlChan)
	}
}
