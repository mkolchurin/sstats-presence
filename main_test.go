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

var (
	Q_setRanked1                                                       = setRankedMode + "?sid=1&rankedMode=true"
	Q_setRanked2                                                       = setRankedMode + "?sid=2&rankedMode=true"
	Q_setRanked1Unranked                                               = setRankedMode + "?sid=1&rankedMode=false"
	Q_setRanked2Unranked                                               = setRankedMode + "?sid=2&rankedMode=false"
	Q_setRanked3Unranked                                               = setRankedMode + "?sid=3&rankedMode=false"
	Q_setRanked3                                                       = setRankedMode + "?sid=3&rankedMode=true"
	Q_getRanked1                                                       = getRankedMode + "?sid=1"
	Q_getRanked2                                                       = getRankedMode + "?sid=2"
	Q_getRanked1_2_3                                                   = getRankedMode + "?sid=1,2,3"
	Q_ping4                                                            = pingRequest + "?sid=4"
	Q_getRanked4                                                       = getRankedMode + "?sid=4"
	Q_ping3                                                            = pingRequest + "?sid=3"
	Q_ping5ModDowstats                                                 = pingRequest + "?sid=5&" + modParam + "=dowstats_balance_mod"
	Q_ping6ModDowstats                                                 = pingRequest + "?sid=6&" + modParam + "=dowstats_balance_mod"
	Q_ping7tournament                                                  = pingRequest + "?sid=7&" + modParam + "=tournamentpatch"
	Q_ping8unknown                                                     = pingRequest + "?sid=8&" + modParam + "=superduperpathmod"
	R_empty                                                            = "[]"
	R_pingZero                                                         = `[{"onlineCount":0, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":0},{"ModName":"` + modTextList[1] + `","OnlineCount":0},{"ModName":"` + modTextList[2] + `","OnlineCount":0},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingNoMod                                                        = `[{"onlineCount":1, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":0},{"ModName":"` + modTextList[2] + `","OnlineCount":0},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingOnlineDxp2_1                                                 = `[{"onlineCount":1, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":0},{"ModName":"` + modTextList[2] + `","OnlineCount":0},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingOnlineDxp2_1_Dowstats_balance_mod_1                          = `[{"onlineCount":2, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":1},{"ModName":"` + modTextList[2] + `","OnlineCount":0},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingOnlineDxp2_2_Dowstats_balance_mod_2                          = `[{"onlineCount":3, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":2},{"ModName":"` + modTextList[2] + `","OnlineCount":0},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingOnlineDxp2_2_Dowstats_balance_mod_2_tournamentpatch_1        = `[{"onlineCount":4, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":2},{"ModName":"` + modTextList[2] + `","OnlineCount":1},{"ModName":"` + modTextList[3] + `","OnlineCount":0}]}]`
	R_pingOnlineDxp2_2_Dowstats_balance_mod_2_tournamentpatch_1_other1 = `[{"onlineCount":5, "modArray": [{"ModName":"` + modTextList[0] + `","OnlineCount":1},{"ModName":"` + modTextList[1] + `","OnlineCount":2},{"ModName":"` + modTextList[2] + `","OnlineCount":1},{"ModName":"` + modTextList[3] + `","OnlineCount":1}]}]`
	R_getRanked4                                                       = `[{"SID":"4","Ranked":true,"Online":true}]`
	R_getRanked1Fast                                                   = `[{"SID":"1","Ranked":true,"Online":true}]`
	R_getRanked2Unranked                                               = `[{"SID":"2","Ranked":false,"Online":true}]`
	R_getRanked4Slow                                                   = `[{"SID":"4","Ranked":true,"Online":false}]`
	R_getRanked1_2_3FastRanked                                         = `[{"SID":"1","Ranked":true,"Online":true},{"SID":"2","Ranked":true,"Online":true},{"SID":"3","Ranked":true,"Online":true}]`
	R_getRanked1_2_3FastUnRanked                                       = `[{"SID":"1","Ranked":false,"Online":true},{"SID":"2","Ranked":false,"Online":true},{"SID":"3","Ranked":false,"Online":true}]`
)

func TestLoadPing(t *testing.T) {
	_ = os.Remove(testDbName)
	r := gin.Default()
	db, err := leveldb.OpenFile(testDbName, nil)
	if err != nil {
		logrus.Errorf("failed to open/create db")
	}

	r.GET(":action", getHandler(db))

	for i := 0; i < 100; i++ {
		httpReqGET(t, r, pingRequest+"?sid="+fmt.Sprint(i), `[{"onlineCount":`+fmt.Sprint(i)+`, "modArray": [{"ModName":"`+modTextList[0]+`","OnlineCount":`+fmt.Sprint(i)+`},{"ModName":"`+modTextList[1]+`","OnlineCount":0},{"ModName":"`+modTextList[2]+`","OnlineCount":0},{"ModName":"`+modTextList[3]+`","OnlineCount":0}]}]`)
		countOnlineUsers(db, len(OnlineCounter))
	}

}
func TestTime(t *testing.T) {
	err := os.RemoveAll(testDbName)
	if err != nil {
		Log.Fatalf("Cannot delete folder '%s'", err.Error())
	}
	r := gin.Default()
	db, err := leveldb.OpenFile(testDbName, nil)
	if err != nil {
		logrus.Errorf("failed to open/create db")
	}
	r.GET("uniq", getUniq(db))
	r.GET(":action", getHandler(db))

	//test
	httpReqGET(t, r, "uniq", "{\"day\": 0, \"month\": 0, \"year\": 0 }")
	httpReqGET(t, r, "uniq", "{\"day\": 0, \"month\": 0, \"year\": 0 }")

	httpReqGET(t, r, Q_ping4, R_pingZero)
	httpReqGET(t, r, "uniq", "{\"day\": 1, \"month\": 1, \"year\": 1 }")
	httpReqGET(t, r, Q_ping4, R_pingZero)
	httpReqGET(t, r, "uniq", "{\"day\": 1, \"month\": 1, \"year\": 1 }")

	httpReqGET(t, r, Q_ping3, R_pingZero)
	httpReqGET(t, r, Q_ping5ModDowstats, R_pingZero)
	httpReqGET(t, r, "uniq", "{\"day\": 3, \"month\": 3, \"year\": 3 }")

}

func TestAPI(t *testing.T) {
	err := os.RemoveAll(testDbName)
	if err != nil {
		Log.Fatalf("Cannot delete folder '%s'", err.Error())
	}

	r := gin.Default()
	db, err := leveldb.OpenFile(testDbName, nil)
	if err != nil {
		logrus.Errorf("failed to open/create db")
	}

	r.GET(":action", getHandler(db))

	httpReqGET(t, r, Q_ping4, R_pingZero)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping4, R_pingNoMod)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping5ModDowstats, R_pingOnlineDxp2_1)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping6ModDowstats, R_pingOnlineDxp2_1_Dowstats_balance_mod_1)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping7tournament, R_pingOnlineDxp2_2_Dowstats_balance_mod_2)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping8unknown, R_pingOnlineDxp2_2_Dowstats_balance_mod_2_tournamentpatch_1)
	countOnlineUsers(db, len(OnlineCounter))
	httpReqGET(t, r, Q_ping4, R_pingOnlineDxp2_2_Dowstats_balance_mod_2_tournamentpatch_1_other1)

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
