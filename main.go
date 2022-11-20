package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"os"
	"sstats-presence/playerStorage"
	"strconv"
	"strings"
	"time"
)

const (
	tokenHeader     = "eyJSb2xlIjoiQWRtaW4iLCJJc3N1ZXIiOiJJc3N1ZXIiLCJVc2VybmFtZSI6IkphdmFJblVzZSIsImV4cCI6MTY2NDI3ODY2NCwF"
	getRankedMode   = "getRankedMode"
	setRankedMode   = "setRankedMode"
	pingRequest     = "pingRequest"
	rankedParam     = "rankedMode"
	counterSleepSec = 5
)

var Log = logrus.New()
var OnlineCounter int

func main() {
	Log.Out = os.Stdout
	Log.SetLevel(logrus.ErrorLevel)

	db, err := leveldb.OpenFile("ps.db", nil)

	go onlineCount(db)

	//setup gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())

	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	r.Use(gin.Recovery())
	pprof.Register(r)
	r.GET(":action", getHandler(db))
	err = r.Run(":8081")
	if err != nil {
		return
	}
	return
}

func getHandler(db *leveldb.DB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		token := c.Request.Header["Token"]
		if (token == nil) || (token[0] != tokenHeader) {
			Log.Errorf("Wrong token '%s'", token)
			c.String(403, "")
			return
		}
		sidsRow, sidExist := c.GetQueryArray("sid")
		action := c.Param("action")
		ranked := c.Query(rankedParam)

		var resp []byte
		resp = nil
		if sidExist {
			sids := strings.Split(sidsRow[0], ",")
			switch action {
			case getRankedMode:
				resp = getRanked(sids, db, resp)
				break
			case setRankedMode:
				err := setRanked(sids, ranked, db)
				if err != nil {
					Log.Errorf("failed to set ranked '%s'", err.Error())
				}
				break
			case pingRequest:
				setPing(db, sids)
				resp = []byte(fmt.Sprintf("[{\"onlineCount\":%d}]", OnlineCounter))
				break
			default:
				Log.Errorf("Action not found '%s'", action)
			}
		} else {
			Log.Errorf("GET: sids not found")
		}
		if resp == nil {
			resp = []byte("[]")
		}
		c.String(200, string(resp))
	}

	return gin.HandlerFunc(fn)
}

func setPing(db *leveldb.DB, sids []string) {
	if len(sids) > 1 {
		Log.Infof("ping: got more than 1 sid, process the first one, '%s'", sids)
	}
	stateTrunc, err := playerStorage.GetFromBase(db, sids[0])
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Debugf(pingRequest + ": sid not found, try create the new one")
		stateTrunc.Ranked = true
	}
	stateTrunc.LastPing = time.Now().Unix()
	encoded, err := stateTrunc.Encode()
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf(pingRequest+": failed to encode '%s'", err.Error())
	}
	err = playerStorage.PutToBase(db, sids[0], encoded)
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf(pingRequest+": save to db is failed '%s'", err.Error())
	}
}

func setRanked(sids []string, ranked string, db *leveldb.DB) error {
	if len(sids) > 1 {
		Log.Infof("sids more than one, apply to the first one")
	}
	rankedBool, err := strconv.ParseBool(ranked)
	stateRecord := playerStorage.PlayerStateRecord{Ranked: rankedBool, LastPing: time.Now().Unix()}
	encodedPlayerState, err := stateRecord.Encode()
	if err != nil {
		return err
	}
	err = playerStorage.PutToBase(db, sids[0], encodedPlayerState)
	if err != nil {
		return err
	}

	Log.Debugf("sid %s; ranked %t; time %s; online %d", sids[0], stateRecord.Ranked, time.Unix(stateRecord.LastPing, 0).String(), OnlineCounter)
	return nil
}

func onlineCount(db *leveldb.DB) {
	for {
		time.Sleep(counterSleepSec * time.Second)
		iter := db.NewIterator(nil, nil)
		onlineCounter := 0
		for iter.Next() {
			value := iter.Value()
			rec, _ := playerStorage.DecodeRecord(value)
			if rec.IsOnline() {
				onlineCounter += 1
			}
		}
		iter.Release()
		err := iter.Error()
		if err != nil {
			Log.Errorf("Failed to iterate db %s", err.Error())
			continue
		}
		OnlineCounter = onlineCounter
	}
}

func getRanked(sids []string, db *leveldb.DB, resp []byte) []byte {
	marshal := []byte("[")
	for i, sid := range sids {
		playerStateTruncDecoded, err := playerStorage.GetFromBase(db, sid)
		if err != nil {
			Log.WithFields(logrus.Fields{
				"sid": sids[0],
			}).Debugf(getRankedMode + ": sid not found, try create the new one")
			err := playerStorage.PutToBaseEmpty(db, sid, &playerStateTruncDecoded)
			if err != nil {
				Log.Errorf(getRankedMode + ": failed to create empty record")
			}
		} else {
			Log.Debugf("found sid %d/%d: '%s'", i+1, len(sids), sid)
		}
		var response playerStorage.PlayerStateResponse
		response.ToPlayerState(sid, playerStateTruncDecoded)
		if !playerStateTruncDecoded.IsOnline() {
			response.Ranked = true
			playerStateTruncDecoded.Ranked = true
			encodedPlayerState, err := playerStateTruncDecoded.Encode()
			if err != nil {
				Log.Errorf("failed to encode ranked")
			}
			err = playerStorage.PutToBase(db, sid, encodedPlayerState)
			if err != nil {
				Log.Errorf("failed to save ranked")
			}
		}
		subString, err := json.Marshal(response)
		if err != nil {
			Log.Errorf("failed json marshal: %s", sid)
			continue
		}
		marshal = append(marshal, subString...)
		if i < (len(sids) - 1) {
			marshal = append(marshal, byte(','))
		}
	}
	if marshal != nil {
		marshal = append(marshal, byte(']'))
		resp = marshal
	}
	return resp
}
