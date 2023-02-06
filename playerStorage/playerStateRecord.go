package playerStorage

import (
	"bytes"
	"encoding/gob"
	"github.com/sirupsen/logrus"
	Log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"time"
)

const (
	OnlineDurationInSec = 5
)

type PlayerStateRecord struct {
	Ranked   bool
	LastPing int64
	Mod      int32
}

func (playerRecord PlayerStateRecord) IsOnline() bool {
	if playerRecord.LastPing == 0 {
		return false
	}
	lastPing := time.Unix(playerRecord.LastPing, 0)
	if lastPing.Add(OnlineDurationInSec*time.Second).Unix() > time.Now().Unix() {
		return true
	}
	return false
}

func (playerRecord PlayerStateRecord) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(playerRecord)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GetFromBase(playersMap map[string][]byte, sid string) (PlayerStateRecord, error) {
	b := dbGet(playersMap, sid)
	if b == nil {
		Log.WithFields(logrus.Fields{
			"sid": sid,
		}).Debugf("get from db nil")
		return PlayerStateRecord{}, errors.ErrNotFound
	}
	playerStateTruncDecoded, err := DecodeRecord(b)
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sid,
		}).Errorf("failed decodeRecord with error '%s'", err.Error())
		return PlayerStateRecord{}, err
	}
	return playerStateTruncDecoded, nil
}

func PutToBase(playersMap map[string][]byte, key string, value []byte) error {
	err := dbPut(playersMap, key, value)
	return err
}

func PutToBaseEmpty(playersMap map[string][]byte, sid string, playerStateTruncDecoded *PlayerStateRecord) error {
	playerStateTruncDecoded.Ranked = true
	playerStateTruncDecoded.LastPing = 0
	encodedPlayerState, err := playerStateTruncDecoded.Encode()
	if err != nil {
		return err
	}
	err = PutToBase(playersMap, sid, encodedPlayerState)
	return err
}

func DecodeRecord(b []byte) (PlayerStateRecord, error) {
	var buf bytes.Buffer
	buf.Write(b)
	dec := gob.NewDecoder(&buf)
	var playerState PlayerStateRecord
	err := dec.Decode(&playerState)
	if err != nil {
		return PlayerStateRecord{}, err
	}
	return playerState, nil
}

func dbPut(playersMap map[string][]byte, key string, value []byte) error {
	playersMap[key] = value
	return nil
}
func dbGet(playersMap map[string][]byte, sid string) []byte {
	b := playersMap[sid]
	return b
}
