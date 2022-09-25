package playerStorage

import (
	"bytes"
	"encoding/gob"
	"github.com/sirupsen/logrus"
	Log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"time"
)

const (
	OnlineDurationInSec = 10
)

type PlayerStateRecord struct {
	Ranked   bool
	LastPing int64
}

type PlayerStateResponse struct {
	SID    string
	Ranked bool
	Online bool
}

func (playerState *PlayerStateResponse) ToPlayerState(sid string, playerStateTruncDecoded PlayerStateRecord) {
	playerState.SID = sid
	playerState.Online = playerStateTruncDecoded.IsOnline()
	playerState.Ranked = playerStateTruncDecoded.Ranked
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

func GetFromBase(db *leveldb.DB, sid string) (PlayerStateRecord, error) {
	b, err := db.Get([]byte(sid), nil)
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sid,
		}).Debugf("get from db with error: '%s'", err.Error())
		return PlayerStateRecord{}, err
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

func PutToBase(db *leveldb.DB, key string, value []byte) error {
	err := db.Put([]byte(key), value, nil)
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
