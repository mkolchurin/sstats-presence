package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/syndtr/goleveldb/leveldb"
	"log"
	"strconv"
	"time"
)

const (
	getRankedMode       = "get"
	setRankedMode       = "set"
	pingRequest         = "ping"
	OnlineDurationInSec = 10
)

//func reqConv(ranked bool, ping bool) byte {
//	oneIfTrue := func(e bool) byte {
//		if e {
//			return 1
//		} else {
//			return 0
//		}
//	}
//	return (oneIfTrue(ranked) << 1) | (oneIfTrue(ping))
//}

type PlayerStateTrunc struct {
	Ranked   bool
	LastPing int64
}

type PlayerState struct {
	SID string
	PlayerStateTrunc
}

func isOnline(ps PlayerStateTrunc) bool {
	if ps.LastPing == 0 {
		return false
	}
	lastPing := time.Unix(ps.LastPing, 0)
	if lastPing.Add(OnlineDurationInSec*time.Second).Unix() > time.Now().Unix() {
		return true
	}
	return false

}

func main() {
	db, err := leveldb.OpenFile("ps.db", nil)
	r := gin.Default()
	pprof.Register(r)
	r.GET(":action", func(c *gin.Context) {
		action := c.Param("action")
		sid := c.Query("sid")
		ranked := c.Query("ranked")
		resp := []byte("nil")
		switch action {
		case getRankedMode:
			b, err := db.Get([]byte(sid), nil)
			if err != nil {
				resp = []byte("nf")
				break
			}
			psDec, err := decode(b)
			if err != nil {
				return
			}
			log.Printf("succeed sid: %s; ranked %t; time %s;"+
				"online %t\n\n", sid, psDec.Ranked, time.Unix(psDec.LastPing, 0).String(), isOnline(psDec))

			ps := PlayerState{
				sid, psDec,
			}

			marshal, err := json.Marshal(ps)
			if err != nil {
				resp = []byte("marshal failed")
			}
			resp = marshal
			break
		case setRankedMode:
			rankedBool, err := strconv.ParseBool(ranked)
			p := PlayerStateTrunc{rankedBool, time.Now().Unix()}

			psBuf, err := encode(p)
			if err != nil {
				resp = []byte("encode failed")
				break
			}
			err = db.Put([]byte(sid), psBuf, nil)
			if err != nil {
				resp = []byte("failed")
				break
			}
			resp = []byte(fmt.Sprintf("succeed sid: %s; ranked %t; time %s", sid, p.Ranked, time.Unix(p.LastPing, 0).String()))
			//rankedBool, err := strconv.ParseBool(ranked)
			//if err != nil {
			//	log.Print(err)
			//}
			//storage[sid] = reqConv(rankedBool, false)
			//resp = []byte(fmt.Sprintf("succeed set ranked %d", storage[sid]))
			break
		case pingRequest:
			//storage[sid] = reqConv(false, true)
			//resp = []byte(fmt.Sprintf("succeed ping %d", storage[sid]))
			break
		}
		c.String(200, string(resp))
	})
	err = r.Run(":8080")
	if err != nil {
		return
	}
	return
}

func encode(state PlayerStateTrunc) ([]byte, error) {
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
