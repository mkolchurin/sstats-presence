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

func PutToBaseEmpty(db *leveldb.DB, sid string, playerStateTruncDecoded *PlayerStateRecord) error {
	playerStateTruncDecoded.Ranked = true
	playerStateTruncDecoded.LastPing = 0
	encodedPlayerState, err := playerStateTruncDecoded.Encode()
	if err != nil {
		return err
	}
	err = PutToBase(db, sid, encodedPlayerState)
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
