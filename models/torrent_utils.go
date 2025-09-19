package models

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"github.com/zeebo/bencode"
	"io"
	"strings"
)

// GetInfoHashFromTorrentData extracts the info hash from torrent file data
func GetInfoHashFromTorrentData(torrentData []byte) (string, error) {
	var torrent map[string]interface{}

	// Decode the bencode data
	decoder := bencode.NewDecoder(strings.NewReader(string(torrentData)))
	if err := decoder.Decode(&torrent); err != nil {
		return "", errors.New("failed to decode torrent data: " + err.Error())
	}

	// Get the info dictionary
	infoDict, ok := torrent["info"].(map[string]interface{})
	if !ok {
		return "", errors.New("info dictionary not found in torrent data")
	}

	// Encode the info dictionary to bencode again
	var buffer strings.Builder
	encoder := bencode.NewEncoder(&buffer)
	if err := encoder.Encode(infoDict); err != nil {
		return "", errors.New("failed to encode info dictionary: " + err.Error())
	}

	// Calculate SHA1 hash of the encoded info dictionary
	h := sha1.New()
	if _, err := io.WriteString(h, buffer.String()); err != nil {
		return "", errors.New("failed to calculate SHA1 hash: " + err.Error())
	}

	// Convert the hash to lowercase hexadecimal
	return strings.ToLower(hex.EncodeToString(h.Sum(nil))), nil
}
