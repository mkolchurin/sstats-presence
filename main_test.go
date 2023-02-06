package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"net/http"
	"net/http/httptest"
	"os"
	"sstats-presence/playerStorage"
	"testing"
	"time"
)

const (
	testDbName = "test.db"
)

const (
	Q_setRanked1         = setRankedMode + "?sid=1&rankedMode=true"
	Q_setRanked2         = setRankedMode + "?sid=2&rankedMode=true"
	Q_setRanked1Unranked = setRankedMode + "?sid=1&rankedMode=false"
	Q_setRanked2Unranked = setRankedMode + "?sid=2&rankedMode=false"
	Q_setRanked3Unranked = setRankedMode + "?sid=3&rankedMode=false"
	Q_setRanked3         = setRankedMode + "?sid=3&rankedMode=true"
	Q_getRanked1         = getRankedMode + "?sid=1"
	Q_getRanked2         = getRankedMode + "?sid=2"
	Q_getRanked1_2_3     = getRankedMode + "?sid=1,2,3"
	Q_ping4              = pingRequest + "?sid=4"
	Q_getRanked4         = getRankedMode + "?sid=4"
	Q_ping3              = pingRequest + "?sid=3"
	Q_ping5ModDowstats   = pingRequest + "?sid=3&" + modParam + "=dowstats_balance_mod"

	R_empty    = "[]"
	R_pingZero = `[{"onlineCount":0, "modArray": [{"ModName":"dxp2","OnlineCount":0},{"ModName":"dowstats_balance_mod","OnlineCount":0},{"ModName":"dournamentpath","OnlineCount":0},{"ModName":"othermod","OnlineCount":0}]}]`
	R_ping3    = `[{"onlineCount":1, "modArray": [{"ModName":"dxp2","OnlineCount":1},{"ModName":"dowstats_balance_mod","OnlineCount":0},{"ModName":"dournamentpath","OnlineCount":0},{"ModName":"othermod","OnlineCount":0}]}]`

	R_getRanked4                 = `[{"SID":"4","Ranked":true,"Online":true}]`
	R_getRanked1Fast             = `[{"SID":"1","Ranked":true,"Online":true}]`
	R_getRanked2Unranked         = `[{"SID":"2","Ranked":false,"Online":true}]`
	R_getRanked4Slow             = `[{"SID":"4","Ranked":true,"Online":false}]`
	R_getRanked1_2_3FastRanked   = `[{"SID":"1","Ranked":true,"Online":true},{"SID":"2","Ranked":true,"Online":true},{"SID":"3","Ranked":true,"Online":true}]`
	R_getRanked1_2_3FastUnRanked = `[{"SID":"1","Ranked":false,"Online":true},{"SID":"2","Ranked":false,"Online":true},{"SID":"3","Ranked":false,"Online":true}]`
)

func TestAPI(t *testing.T) {
	_ = os.Remove(testDbName)
	r := gin.Default()
	db, err := leveldb.OpenFile(testDbName, nil)
	if err != nil {
		logrus.Errorf("failed to open/create db")
	}
	go countOnlineUsers(db, len(OnlineCounter))
	r.GET(":action", getHandler(db))

	httpReqGET(t, r, Q_ping4, R_pingZero)
	time.Sleep(1)
	httpReqGET(t, r, Q_ping3, R_ping3)
	httpReqGET(t, r, Q_ping3, R_ping3)
	httpReqGET(t, r, Q_ping5ModDowstats, R_ping3)

	//time.Sleep(2)

	httpReqGET(t, r, Q_getRanked4, R_getRanked4)
	httpReqGET(t, r, Q_setRanked1, R_empty)
	httpReqGET(t, r, Q_setRanked2, R_empty)
	httpReqGET(t, r, Q_setRanked3, R_empty)
	httpReqGET(t, r, Q_getRanked1, R_getRanked1Fast)
	httpReqGET(t, r, Q_getRanked1_2_3, R_getRanked1_2_3FastRanked)
	httpReqGET(t, r, Q_setRanked2Unranked, R_empty)
	httpReqGET(t, r, Q_getRanked2, R_getRanked2Unranked)
	httpReqGET(t, r, Q_setRanked1Unranked, R_empty)
	httpReqGET(t, r, Q_setRanked3Unranked, R_empty)
	httpReqGET(t, r, Q_getRanked1_2_3, R_getRanked1_2_3FastUnRanked)

	//test online status
	c := make(chan bool)
	go func() {
		time.Sleep(playerStorage.OnlineDurationInSec * time.Second)
		httpReqGET(t, r, Q_getRanked4, R_getRanked4Slow)
		c <- true
	}()

	//await online check end
	<-c
	return
}

func httpReqGET(t *testing.T, r *gin.Engine, url string, response string) {
	req, _ := http.NewRequest("GET", "/"+url, nil)
	req.Header = map[string][]string{
		"Token": {tokenHeader},
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	httpResp := w.Body.Bytes()
	assert.Equal(t, string(httpResp), response)
	assert.Equal(t, w.Code, http.StatusOK)
	fmt.Println(string(httpResp))
}
