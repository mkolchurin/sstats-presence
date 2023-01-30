package playerStorage

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
