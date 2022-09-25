package main

import (
	"encoding/json"
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
	getRankedMode = "getRankedMode"
	setRankedMode = "setRankedMode"
	pingRequest   = "pingRequest"
	rankedParam   = "rankedMode"
)

var Log = logrus.New()

func main() {
	Log.Out = os.Stdout
	Log.SetLevel(logrus.DebugLevel)

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
				err = setRanked(sids, ranked, db)
				if err != nil {
					Log.Errorf("failed to set ranked '%s'", err.Error())
				}
				break
			case pingRequest:
				setPing(db, sids)
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
	})
	err = r.Run(":8080")
	if err != nil {
		return
	}
	return
}

func setPing(db *leveldb.DB, sids []string) {
	if len(sids) > 1 {
		Log.Infof("ping: got more than 1 sid, process the first one, '%s'", sids)
	}
	stateTrunc, err := playerStorage.GetFromBase(db, sids[0])
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Debugf("ping: sid not found, try create the new one")
	}
	stateTrunc.LastPing = time.Now().Unix()
	encoded, err := stateTrunc.Encode()
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf("ping: failed to encode '%s'", err.Error())
	}
	err = playerStorage.PutToBase(db, sids[0], encoded)
	if err != nil {
		Log.WithFields(logrus.Fields{
			"sid": sids[0],
		}).Errorf("ping: save to db is failed '%s'", err.Error())
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
	Log.Debugf("%s; ranked %t; time %s", sids[0], stateRecord.Ranked, time.Unix(stateRecord.LastPing, 0).String())
	return nil
}

func getRanked(sids []string, db *leveldb.DB, resp []byte) []byte {
	marshal := []byte("[")
	for i, sid := range sids {
		playerStateTruncDecoded, err := playerStorage.GetFromBase(db, sid)
		if err != nil {
			continue
		}
		Log.Debugf("found sid %d/%d: '%s'", i+1, len(sids), sid)

		var response playerStorage.PlayerStateResponse
		response.ToPlayerState(sid, playerStateTruncDecoded)
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
		marshal[len(marshal)-1] = ']'
		resp = marshal
	}
	return resp
}
