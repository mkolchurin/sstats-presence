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
	getRankedMode       = "get"
	setRankedMode       = "set"
	pingRequest         = "ping"
	OnlineDurationInSec = 10
)

type PlayerStateTrunc struct {
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
		ranked := c.Query("ranked")
		var resp []byte
		resp = nil
		if sidExist {
			switch action {
			case getRankedMode:
				resp = getRanked(sids, db, resp)
				break
			case setRankedMode:
				resp, err = setRanked(sids, ranked, db, resp)
				if err != nil {
					log.Errorf("failed to set ranked '%s'", err.Error())
				}
				break
			case pingRequest:
				if len(sids) > 1 {
					log.Infof("got more than 1 sid, process the first one, '%s'", sids)
				}
				stateTrunc, err := getFromDb(db, sids[0])
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
				err = saveToDb(db, sids[0], encoded)
				if err != nil {
					log.WithFields(logrus.Fields{
						"sid": sids[0],
					}).Errorf("ping: save to db is failed '%s'", err.Error())
				}
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

func setRanked(sids []string, ranked string, db *leveldb.DB, resp []byte) ([]byte, error) {
	if len(sids) > 1 {
		log.Infof("sids more than one, apply to the first one")
	}
	rankedBool, err := strconv.ParseBool(ranked)
	p := PlayerStateTrunc{rankedBool, time.Now().Unix()}
	encodedPlayerState, err := p.encode()
	if err != nil {
		return nil, err
	}
	err = saveToDb(db, sids[0], encodedPlayerState)
	if err != nil {
		return nil, err
	}
	log.Debugf("%s; ranked %t; time %s", sids[0], p.Ranked, time.Unix(p.LastPing, 0).String())
	return resp, nil
}

func saveToDb(db *leveldb.DB, key string, value []byte) error {
	err := db.Put([]byte(key), value, nil)
	return err
}

func getRanked(sids []string, db *leveldb.DB, resp []byte) []byte {
	marshal := []byte("[")
	for i, sid := range sids {
		playerStateTruncDecoded, err := getFromDb(db, sid)
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

func (playerState *PlayerState) toPlayerState(sid string, playerStateTruncDecoded PlayerStateTrunc) {
	playerState.SID = sid
	playerState.Online = playerStateTruncDecoded.isOnline()
	playerState.Ranked = playerStateTruncDecoded.Ranked
}

func getFromDb(db *leveldb.DB, sid string) (PlayerStateTrunc, error) {
	b, err := db.Get([]byte(sid), nil)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sid,
		}).Debugf("get from db with error: '%s'", err.Error())
		return PlayerStateTrunc{}, err
	}
	playerStateTruncDecoded, err := decode(b)
	if err != nil {
		log.WithFields(logrus.Fields{
			"sid": sid,
		}).Errorf("failed decode with error '%s'", err.Error())
		return PlayerStateTrunc{}, err
	}
	return playerStateTruncDecoded, nil
}

func (ps PlayerStateTrunc) isOnline() bool {
	if ps.LastPing == 0 {
		return false
	}
	lastPing := time.Unix(ps.LastPing, 0)
	if lastPing.Add(OnlineDurationInSec*time.Second).Unix() > time.Now().Unix() {
		return true
	}
	return false
}

func (state PlayerStateTrunc) encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(state)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(b []byte) (PlayerStateTrunc, error) {
	var buf bytes.Buffer
	buf.Write(b)
	dec := gob.NewDecoder(&buf)
	var playerState PlayerStateTrunc
	err := dec.Decode(&playerState)
	if err != nil {
		return PlayerStateTrunc{}, err
	}
	return playerState, nil
}

//db, err := bolt.Open("presence.db", 0600, nil)
//if err != nil {
//	log.Fatal(err)
//}
//
//err = db.Update(func(tx *bolt.Tx) error {
//	str := Storage{}
//	str.Ping = true
//	str.Presence = false
//	bucket, err := tx.CreateBucket([]byte("players"))
//	if err != nil {
//		return fmt.Errorf("create bucket: %s", err)
//	}
//	buf, _ := json.Marshal(str)
//	err = bucket.Put([]byte("1234"), buf)
//	if err != nil {
//		return err
//	}
//	return nil
//})
//err = db.View(func(tx *bolt.Tx) error {
//	bucket := tx.Bucket([]byte("players"))
//	c := bucket.Cursor()
//
//	for k, v := c.First(); k != nil; k, v = c.Next() {
//		fmt.Printf("key=%s, value=%s\n", k, v)
//	}
//	return nil
//})
//defer db.Close()
