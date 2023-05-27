package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kinozaltv_monitor/config"
	"kinozaltv_monitor/kinozal"
	"net/http"
	"text/template"
)

const messageTorrentAdded = `<b>Добавлен новый торрент</b>
<b>Название:</b> {{ .Title }}
<b>Хеш:</b> {{ .Hash }}
<b>Ссылка:</b> {{ .Url }}`

const messageTorrentUpdated = `<b>Обновлен торрент</b>
<b>Название:</b> {{ .Title }}
<b>Хеш:</b> {{ .Hash }}
<b>Ссылка:</b> {{ .Url }}`

var globalConfig = config.GlobalConfig

// BaseChat is a base chat interface
type BaseChat struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// NewBaseChat creates a new base chat
func NewBaseChat(chatID string, text string) BaseChat {
	return BaseChat{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}
}

func SendCommand(token string, m BaseChat) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}()
	return nil
}

// SendTorrentAdded sends a message about added torrent
func SendTorrentAction(action, token string, torrentInfo kinozal.KinozalTorrent) error {
	var tpl bytes.Buffer

	switch action {
	case "added":
		t, err := template.New("added_torrent").Parse(messageTorrentAdded)
		if err != nil {
			return err
		}
		err = t.Execute(&tpl, torrentInfo)
		if err != nil {
			return err
		}
	case "updated":
		t, err := template.New("updated_torrent").Parse(messageTorrentUpdated)
		if err != nil {
			return err
		}
		err = t.Execute(&tpl, torrentInfo)
		if err != nil {
			return err
		}
	}

	m := NewBaseChat(globalConfig.TelegramChatId, tpl.String())

	return SendCommand(token, m)
}
