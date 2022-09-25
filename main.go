package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"os"
	"strings"

	"strconv"
	"time"
)

const (
	getRankedMode       = "getRankedMode"
	setRankedMode       = "setRankedMode"
	pingRequest         = "pingRequest"
	OnlineDurationInSec = 10
	rankedParam         = "rankedMode"
)

type PlayerStateRecord struct {
	Ranked   bool
	LastPing int64
}

type PlayerState struct {
	SID    string
	Ranked bool
	Online bool
}

var log = logrus.New()

func main() {
	log.Out = os.Stdout
	log.SetLevel(logrus.DebugLevel)

	db, err := leveldb.OpenFile("ps.db", nil)
	r := gin.Default()
	pprof.Register(r)
	r.GET(":action", func(c *gin.Context) {
		sidsRow, sidExist := c.GetQueryArray("sid")
		sids := strings.Split(sidsRow[0], ",")
		action := c.Param("action")
		ranked := c.Query(rankedParam)
		var resp []byte
		resp = nil
		if sidExist {
			switch action {
			case getRankedMode:
				resp = getRanked(sids, db, resp)
				break
			case setRankedMode:
				err = setRanked(sids, ranked, db, resp)
				if err != nil {
					log.Errorf("failed to set ranked '%s'", err.Error())
				}
				break
			case pingRequest:
				setPing(db, sids)
				break
			default:
				log.Errorf("Action not found '%s'", action)
			}
		} else {
			log.Errorf("GET: sids not found")
		}
		if resp == nil {
			resp = []byte("[]")
		}
		c.String(200, string(resp))
	})
	err = r.Run(":8080")
	if err != nil {
		return
	}
	return
}

func setPing(db *leveldb.DB, sids []string) {
	if len(sids) > 1 {
		log.Infof("ping: got more than 1 sid, process the first one, '%s'", sids)
	}
	stateTrunc, err := getFromBase(db, sids[0])
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Debugf("ping: sid not found, try create the new one")
	}
	stateTrunc.LastPing = time.Now().Unix()
	encoded, err := stateTrunc.encode()
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf("ping: failed to encode '%s'", err.Error())
	}
	err = putToBase(db, sids[0], encoded)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf("ping: save to db is failed '%s'", err.Error())
	}
}

func setRanked(sids []string, ranked string, db *leveldb.DB, resp []byte) error {
	if len(sids) > 1 {
		log.Infof("sids more than one, apply to the first one")
	}
	rankedBool, err := strconv.ParseBool(ranked)
	stateRecord := PlayerStateRecord{rankedBool, time.Now().Unix()}
	encodedPlayerState, err := stateRecord.encode()
	if err != nil {
		return err
	}
	err = putToBase(db, sids[0], encodedPlayerState)
	if err != nil {
		return err
	}
	log.Debugf("%s; ranked %t; time %s", sids[0], stateRecord.Ranked, time.Unix(stateRecord.LastPing, 0).String())
	return nil
}

func getRanked(sids []string, db *leveldb.DB, resp []byte) []byte {
	marshal := []byte("[")
	for i, sid := range sids {
		playerStateTruncDecoded, err := getFromBase(db, sid)
		if err != nil {
			continue
		}
		log.Debugf("found sid %d/%d: '%s'", i+1, len(sids), sid)

		var ps PlayerState
		ps.toPlayerState(sid, playerStateTruncDecoded)
		subString, err := json.Marshal(ps)
		if err != nil {
			log.Errorf("failed json marshal: %s", sid)
			continue
		}
		marshal = append(marshal, subString...)
		if i < (len(sids) - 1) {
			marshal = append(marshal, byte(','))
		}
	}
	if marshal != nil {
		marshal[len(marshal)-1] = ']'
		resp = marshal
	}
	return resp
}

func (playerState *PlayerState) toPlayerState(sid string, playerStateTruncDecoded PlayerStateRecord) {
	playerState.SID = sid
	playerState.Online = playerStateTruncDecoded.isOnline()
	playerState.Ranked = playerStateTruncDecoded.Ranked
}

func getFromBase(db *leveldb.DB, sid string) (PlayerStateRecord, error) {
	b, err := db.Get([]byte(sid), nil)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sid,
		}).Debugf("get from db with error: '%s'", err.Error())
		return PlayerStateRecord{}, err
	}
	playerStateTruncDecoded, err := decodeRecord(b)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sid,
		}).Errorf("failed decodeRecord with error '%s'", err.Error())
		return PlayerStateRecord{}, err
	}
	return playerStateTruncDecoded, nil
}

func putToBase(db *leveldb.DB, key string, value []byte) error {
	err := db.Put([]byte(key), value, nil)
	return err
}

func (playerRecord PlayerStateRecord) isOnline() bool {
	if playerRecord.LastPing == 0 {
		return false
	}
	lastPing := time.Unix(playerRecord.LastPing, 0)
	if lastPing.Add(OnlineDurationInSec*time.Second).Unix() > time.Now().Unix() {
		return true
	}
	return false
}

func (playerRecord PlayerStateRecord) encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(playerRecord)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeRecord(b []byte) (PlayerStateRecord, error) {
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
