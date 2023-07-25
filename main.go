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
	"sync/atomic"
	_ "sync/atomic"
	"time"
)

const (
	tokenHeader     = ""
	getRankedMode   = "getRankedMode"
	setRankedMode   = "setRankedMode"
	pingRequest     = "pingRequest"
	rankedParam     = "rankedMode"
	modParam        = "gameMod"
	counterSleepSec = 5
)

var modList = []string{"dxp2", "dowstats_balance_mod", "tournamentpatch", "othermod"}
var modTextList = []string{"Dawn of War - Soulstorm", "DoW Stats Balance Mod", "TournamentPatch", "Other mods"}
var OnlineCounter []int32

var Log = logrus.New()

func init() {
	OnlineCounter = make([]int32, len(modList))
}

func main() {
	Log.Out = os.Stdout
	Log.SetLevel(logrus.ErrorLevel)

	db, err := leveldb.OpenFile("ps.db", nil)
	go func() {
		for {
			countOnlineUsers(db, len(OnlineCounter))
			time.Sleep(counterSleepSec * time.Second)
		}
	}()

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
		gameMod := c.Query(modParam)
		var resp []byte
		resp = nil
		if sidExist {
			sids := strings.Split(sidsRow[0], ",")
			switch action {
			case getRankedMode:
				resp = processGetRanked(sids, db, resp)
				break
			case setRankedMode:
				err := processSetRanked(sids, ranked, db)
				if err != nil {
					Log.Errorf("failed to set ranked '%s'", err.Error())
				}
				break
			case pingRequest:
				processPingReq(db, sids, gameMod)
				resp = []byte(onlineUsersResponse())
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

func getModInt(modName string) int {

	if modName == "" {
		return 0
	}
	for i, v := range modList {
		if v == modName {
			return i
		}
	}
	return int(len(modList) - 1)
}

func processPingReq(db *leveldb.DB, sids []string, gameMod string) {
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
	stateTrunc.Mod = int32(getModInt(gameMod))
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

func processSetRanked(sids []string, ranked string, db *leveldb.DB) error {
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

	Log.Debugf("sid %s; ranked %t; time %s; online %d", sids[0], stateRecord.Ranked, time.Unix(stateRecord.LastPing, 0).String(), OnlineCounter[0])
	return nil
}

// show total online players
func countOnlineUsers(db *leveldb.DB, modsCount int) {
	LocalCounter := make([]int32, modsCount)
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		value := iter.Value()
		rec, _ := playerStorage.DecodeRecord(value)
		if rec.IsOnline() {
			LocalCounter[rec.Mod] += 1
		}
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		Log.Errorf("Failed to iterate db %s", err.Error())
		return
	}
	for i := 0; i < modsCount; i++ {
		atomic.StoreInt32(&OnlineCounter[i], LocalCounter[i])
	}
}

func onlineUsersResponse() string {
	var totalOnline int32 = 0
	resp := ""
	for i := 0; i < len(OnlineCounter); i++ {
		totalOnline += OnlineCounter[i]
		s, err := json.Marshal(struct {
			ModName     string
			OnlineCount int
		}{
			ModName: func(modInd int) string {
				//if modInd >= len(modList) {
				//	return "othermod"
				//}
				return modTextList[modInd]
			}(i),
			OnlineCount: int(OnlineCounter[i]),
		})
		if err != nil {
			Log.Errorf("Failed to parse %s", err.Error())
			continue
		}
		resp += string(s) + ","
	}
	resp = strings.TrimSuffix(resp, ",")
	return fmt.Sprintf(`[{"onlineCount":%d, "modArray": [%s]}]`, totalOnline, resp)

}

func processGetRanked(sids []string, db *leveldb.DB, resp []byte) []byte {
	marshal := []byte("[")
	for i, sid := range sids {
		playerStateRecord, err := playerStorage.GetFromBase(db, sid)
		if err != nil {
			Log.WithFields(logrus.Fields{
				"sid": sids[0],
			}).Debugf(getRankedMode + ": sid not found, try create the new one")
			err := playerStorage.PutToBaseEmpty(db, sid, &playerStateRecord)
			if err != nil {
				Log.Errorf(getRankedMode + ": failed to create empty record")
			}
		} else {
			Log.Debugf("found sid %d/%d: '%s'", i+1, len(sids), sid)
		}
		var response playerStorage.PlayerStateResponse
		response.ToPlayerState(sid, playerStateRecord)
		if !playerStateRecord.IsOnline() {
			response.Ranked = true
			playerStateRecord.Ranked = true
			encodedPlayerState, err := playerStateRecord.Encode()
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
