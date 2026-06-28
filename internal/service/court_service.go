package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

func GetSessionView(ctx context.Context, sessionID string) (*model.SessionView, error) {
	session, err := repository.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	AutoCloseIfExpired(ctx, session) // 超過結束時間 2 小時自動關團
	players, err := repository.GetSessionPlayers(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// build player lookup (name + level)
	playerMap := make(map[string]model.SessionPlayer, len(players))
	for _, p := range players {
		playerMap[p.PlayerID] = p
	}

	courts, err := repository.GetCourts(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	views := make([]model.CourtView, 0, len(courts))
	for _, c := range courts {
		canUndo := false
		if c.LastEnd != nil {
			if t, e := time.Parse(time.RFC3339, c.LastEnd.EndedAt); e == nil && time.Since(t) <= 10*time.Minute {
				canUndo = true
			}
		}
		votes := votesStillPlaying(c.EndVotes, c.Playing)
		cv := model.CourtView{
			CourtID:        c.CourtID,
			CourtNum:       courtNum(c.CourtID),
			Name:           c.Name,
			Status:         c.Status,
			Playing:        toPlayingSlots(c.Playing, playerMap),
			Queue:          toSlots(c.Queue, playerMap),
			StartedAt:      c.StartedAt,
			CanUndo:        canUndo,
			EndVotes:       votes,
			EndVotesNeeded: EndVoteThreshold,
		}
		views = append(views, cv)
	}

	return &model.SessionView{
		SessionID:   session.SessionID,
		Title:       session.Title,
		NumCourts:   session.NumCourts,
		Status:      string(session.Status),
		StartAt:     session.StartAt,
		EndAt:       session.EndAt,
		QueueOpenAt: session.QueueOpenAt,
		Courts:      views,
	}, nil
}

// playerInAnyCourt returns the court_id the player is currently in (playing or
// queue), or "" if none — a player may only occupy one court at a time.
func playerInAnyCourt(ctx context.Context, sessionID, playerID string) (string, error) {
	courts, err := repository.GetCourts(ctx, sessionID)
	if err != nil {
		return "", err
	}
	for _, c := range courts {
		if contains(c.Playing, playerID) || contains(c.Queue, playerID) {
			return c.CourtID, nil
		}
	}
	return "", nil
}

// validatePlayer ensures the player_id is a real member of this session,
// so attackers can't inject arbitrary IDs onto courts.
func validatePlayer(ctx context.Context, sessionID, playerID string) error {
	players, err := repository.GetSessionPlayers(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, p := range players {
		if p.PlayerID == playerID {
			return nil
		}
	}
	return fmt.Errorf("player not in this session")
}

// checkQueueOpen blocks player self-service (join playing / queue) before the
// leader's configured queue-open time. Leaders bypass this via admin actions.
func checkQueueOpen(ctx context.Context, sessionID string) error {
	session, err := repository.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.QueueOpenAt == "" {
		return nil // no gate configured
	}
	openAt, err := time.Parse(time.RFC3339, session.QueueOpenAt)
	if err != nil {
		return nil // unparseable → don't block
	}
	if time.Now().Before(openAt) {
		return fmt.Errorf("排隊尚未開放,請於 %s 後再試", openAt.Local().Format("15:04"))
	}
	return nil
}

// updateCourt runs a read-modify-write against one court under an optimistic
// lock. apply mutates the freshly-read court in place; if a concurrent writer
// committed first, the conditional PutCourt is rejected and we re-read & re-apply
// (so two people grabbing the same court can't silently overwrite each other).
// apply may return a business error (e.g. "位置已經有人了") to abort without saving;
// that is returned as-is and NOT retried.
func updateCourt(ctx context.Context, sessionID, courtID string, apply func(*model.Court) error) error {
	const maxTries = 6
	for try := 0; try < maxTries; try++ {
		court, err := repository.GetCourt(ctx, sessionID, courtID)
		if err != nil {
			return err
		}
		if err := apply(court); err != nil {
			return err
		}
		err = repository.PutCourt(ctx, *court)
		if err == nil {
			return nil
		}
		if repository.IsConflict(err) {
			continue // someone wrote between our read and write — re-read & retry
		}
		return err
	}
	return fmt.Errorf("系統忙碌中,請再試一次")
}

// JoinPlaying seats a player at a specific position (0=左上,1=右上,2=左下,3=右下)
func JoinPlaying(ctx context.Context, sessionID, courtID, playerID string, position int) error {
	if position < 0 || position > 3 {
		return fmt.Errorf("invalid position")
	}
	if err := validatePlayer(ctx, sessionID, playerID); err != nil {
		return err
	}
	if err := checkQueueOpen(ctx, sessionID); err != nil {
		return err
	}
	// already in another court? block. already in THIS court? this is a move.
	if cid, _ := playerInAnyCourt(ctx, sessionID, playerID); cid != "" && cid != courtID {
		return fmt.Errorf("你已經在其他場地了,請先退出")
	}
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Playing = normPlaying(court.Playing)
		if court.Playing[position] != "" {
			return fmt.Errorf("這個位置已經有人了")
		}
		// leave any current spot in this court (moving position / promoting from queue)
		clearSlot(court.Playing, playerID)
		court.Queue = remove(court.Queue, playerID)
		court.Playing[position] = playerID
		court.Status = model.CourtPlaying
		// 開打計時只在「湊滿 4 人」那一刻才起算;1~3 人是湊人中,不計時
		if playingCount(court.Playing) == 4 {
			court.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return nil
	})
}

// JoinQueue adds a player to the queue if there's room (< 4) and playing is full
func JoinQueue(ctx context.Context, sessionID, courtID, playerID string) error {
	if err := validatePlayer(ctx, sessionID, playerID); err != nil {
		return err
	}
	if err := checkQueueOpen(ctx, sessionID); err != nil {
		return err
	}
	if cid, _ := playerInAnyCourt(ctx, sessionID, playerID); cid != "" {
		return fmt.Errorf("你已經在其他場地了,請先退出")
	}
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		if len(court.Queue) >= 4 {
			return fmt.Errorf("queue is full")
		}
		if contains(court.Playing, playerID) || contains(court.Queue, playerID) {
			return fmt.Errorf("已經在這個場地了")
		}
		court.Queue = append(court.Queue, playerID)
		recomputeStatus(court) // 純排隊:留在排隊,不自動上場
		return nil
	})
}

// LeaveQueue removes a player from a court's queue
func LeaveQueue(ctx context.Context, sessionID, courtID, playerID string) error {
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Queue = remove(court.Queue, playerID)
		return nil
	})
}

// LeavePlaying lets a player leave a court that is still gathering (< 4).
// Once it's full (the game has started) only the leader can move people.
func LeavePlaying(ctx context.Context, sessionID, courtID, playerID string) error {
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Playing = normPlaying(court.Playing)
		if playingCount(court.Playing) >= 4 {
			return fmt.Errorf("比賽已開始,請找團主")
		}
		clearSlot(court.Playing, playerID)
		recomputeStatus(court) // 留下空位給人選,不自動補
		return nil
	})
}

// creditFinishedGame gives everyone currently playing on the court +1 game and
// their minutes, and writes a GameLog. Shared by EndCourt and DeleteCourt.
func creditFinishedGame(ctx context.Context, sessionID string, court *model.Court, endedAtID string) {
	if playingCount(court.Playing) == 0 {
		return
	}
	now := time.Now().UTC()
	minutes := elapsedMinutes(court.StartedAt)
	players, _ := repository.GetSessionPlayers(ctx, sessionID)
	pm := make(map[string]model.SessionPlayer, len(players))
	for _, p := range players {
		pm[p.PlayerID] = p
	}
	names := make([]string, 0, len(court.Playing))
	for _, pid := range court.Playing {
		if p, ok := pm[pid]; ok {
			p.Games++
			p.TotalMinutes += minutes
			_ = repository.PutSessionPlayer(ctx, p)
			names = append(names, p.DisplayName)
		}
	}
	_ = repository.PutGameLog(ctx, model.GameLog{
		SessionID:   sessionID,
		EndedAtID:   endedAtID,
		CourtNum:    courtNum(court.CourtID),
		PlayerNames: names,
		StartedAt:   court.StartedAt,
		EndedAt:     now.Format(time.RFC3339),
		Minutes:     minutes,
	})
}

func elapsedMinutes(startedAt string) int {
	if startedAt == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
		return int(time.Since(t).Minutes())
	}
	return 0
}

func nonEmptyIDs(p []string) []string {
	out := []string{}
	for _, s := range p {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// EndCourt rotates: playing → cleared, queue → playing.
// Everyone who was playing gets credited one game.
// EndVoteThreshold is how many on-court players must vote before a game
// auto-ends (so players can end a finished game without bugging the leader).
const EndVoteThreshold = 3

// votesStillPlaying keeps only the votes cast by players who are currently on
// the court (a voter who left / got moved no longer counts).
func votesStillPlaying(votes, playing []string) []string {
	if len(votes) == 0 {
		return nil
	}
	out := make([]string, 0, len(votes))
	for _, v := range votes {
		if contains(playing, v) {
			out = append(out, v)
		}
	}
	return out
}

// VoteEndCourt toggles a playing player's vote to end the current game. When the
// vote count reaches the threshold the court is ended automatically. Returns the
// up-to-date (ended, voteCount) so the caller can broadcast/respond.
func VoteEndCourt(ctx context.Context, sessionID, courtID, playerID string) (ended bool, count int, err error) {
	trigger := false
	err = updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		if !contains(court.Playing, playerID) {
			return fmt.Errorf("只有場上的人可以投票結束")
		}
		if playingCount(court.Playing) < 4 {
			return fmt.Errorf("湊滿四人開打後才能投票結束")
		}
		// toggle this player's vote, then drop any stale votes (people who left)
		if contains(court.EndVotes, playerID) {
			court.EndVotes = remove(court.EndVotes, playerID)
		} else {
			court.EndVotes = append(court.EndVotes, playerID)
		}
		court.EndVotes = votesStillPlaying(court.EndVotes, court.Playing)
		count = len(court.EndVotes)
		trigger = count >= EndVoteThreshold
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	if trigger {
		if err := EndCourt(ctx, sessionID, courtID); err != nil {
			return false, count, err
		}
		return true, count, nil
	}
	return false, count, nil
}

func EndCourt(ctx context.Context, sessionID, courtID string) error {
	// snapshot of who was playing (for crediting) + the post-rotation court, both
	// captured on the attempt that actually commits — so a concurrent write that
	// forces a retry never double-credits the finished game.
	endedAtID := time.Now().UTC().Format(time.RFC3339) + "#" + uuid.New().String()
	var finished model.Court
	var promoted []string
	err := updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		finished = *court // Playing slice still points at the old players
		// snapshot pre-end state onto the court so the leader can undo a misclick
		court.LastEnd = &model.EndSnapshot{
			Playing:   normPlaying(court.Playing),
			Queue:     append([]string{}, court.Queue...),
			StartedAt: court.StartedAt,
			EndedAt:   time.Now().UTC().Format(time.RFC3339),
			GameLogID: endedAtID,
			Credited:  nonEmptyIDs(normPlaying(court.Playing)),
			Minutes:   elapsedMinutes(court.StartedAt),
		}
		court.Playing = make([]string, 4) // 清空場上
		court.StartedAt = ""
		court.EndVotes = nil // 新的一場,清空結束投票
		promoted = fillFromQueue(court) // 換排隊的人上場(缺幾補幾)
		return nil
	})
	if err != nil {
		return err
	}
	creditFinishedGame(ctx, sessionID, &finished, endedAtID) // side effects ONCE, after commit
	pushPromoted(ctx, &finished, promoted)
	return nil
}

// UndoEndCourt reverses the last 結束場地 on a court (within 10 min): restores the
// players, queue and 開打計時, and rolls back the credited games / GameLog.
func UndoEndCourt(ctx context.Context, sessionID, courtID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	snap := court.LastEnd
	if snap == nil {
		return fmt.Errorf("沒有可復原的結束")
	}
	if t, e := time.Parse(time.RFC3339, snap.EndedAt); e == nil && time.Since(t) > 10*time.Minute {
		return fmt.Errorf("超過可復原時間(10 分鐘)")
	}
	// reverse the credit applied at end time
	players, _ := repository.GetSessionPlayers(ctx, sessionID)
	pm := make(map[string]model.SessionPlayer, len(players))
	for _, p := range players {
		pm[p.PlayerID] = p
	}
	for _, pid := range snap.Credited {
		if p, ok := pm[pid]; ok {
			if p.Games > 0 {
				p.Games--
			}
			if p.TotalMinutes -= snap.Minutes; p.TotalMinutes < 0 {
				p.TotalMinutes = 0
			}
			_ = repository.PutSessionPlayer(ctx, p)
		}
	}
	_ = repository.DeleteGameLog(ctx, sessionID, snap.GameLogID)
	// restore the court (one-shot: clear LastEnd)
	return updateCourt(ctx, sessionID, courtID, func(c *model.Court) error {
		c.Playing = normPlaying(snap.Playing)
		c.Queue = append([]string{}, snap.Queue...)
		c.StartedAt = snap.StartedAt
		c.LastEnd = nil
		recomputeStatus(c)
		return nil
	})
}

// RenameCourt sets a court's custom name (leader)
func RenameCourt(ctx context.Context, sessionID, courtID, name string) error {
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Name = name
		return nil
	})
}

// RemoveCourt deletes a court (leader). Players currently playing are credited
// into the stats (as if the game ended); queued players are simply dropped.
func RemoveCourt(ctx context.Context, sessionID, courtID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	endedAtID := time.Now().UTC().Format(time.RFC3339) + "#" + uuid.New().String()
	creditFinishedGame(ctx, sessionID, court, endedAtID) // 場上的人計入統計
	return repository.DeleteCourt(ctx, sessionID, courtID)
}

// RemoveSessionPlayer fully disconnects a person: pulls them off every court,
// deletes their push subscription, and removes the session-player record so
// their device's X-Player-ID becomes invalid (can't act anymore).
func RemoveSessionPlayer(ctx context.Context, sessionID, playerID string) error {
	courts, _ := repository.GetCourts(ctx, sessionID)
	for _, c := range courts {
		if contains(c.Playing, playerID) || contains(c.Queue, playerID) {
			_ = updateCourt(ctx, sessionID, c.CourtID, func(court *model.Court) error {
				court.Playing = normPlaying(court.Playing)
				clearSlot(court.Playing, playerID)
				court.Queue = remove(court.Queue, playerID)
				recomputeStatus(court)
				return nil
			})
		}
	}
	_ = repository.DeletePushSub(ctx, playerID)
	return repository.DeleteSessionPlayer(ctx, sessionID, playerID)
}

// KickPlayer removes a player from any state in a court (admin only)
func KickPlayer(ctx context.Context, sessionID, courtID, playerID string) error {
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Playing = normPlaying(court.Playing)
		clearSlot(court.Playing, playerID)
		court.Queue = remove(court.Queue, playerID)
		recomputeStatus(court)
		return nil
	})
}

// removeFromOtherCourts pulls a player out of every court except keepCourtID,
// enforcing one court per player.
func removeFromOtherCourts(ctx context.Context, sessionID, playerID, keepCourtID string) {
	courts, err := repository.GetCourts(ctx, sessionID)
	if err != nil {
		return
	}
	for _, c := range courts {
		if c.CourtID == keepCourtID {
			continue
		}
		if contains(c.Playing, playerID) || contains(c.Queue, playerID) {
			_ = updateCourt(ctx, sessionID, c.CourtID, func(court *model.Court) error {
				court.Playing = normPlaying(court.Playing)
				clearSlot(court.Playing, playerID)
				court.Queue = remove(court.Queue, playerID)
				recomputeStatus(court)
				return nil
			})
		}
	}
}

// AdminAddToPlaying force-adds a player to the first empty slot (admin, bypasses queue)
func AdminAddToPlaying(ctx context.Context, sessionID, courtID, playerID string) error {
	pre, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if playingCount(normPlaying(pre.Playing)) >= 4 {
		return fmt.Errorf("court playing is full")
	}
	removeFromOtherCourts(ctx, sessionID, playerID, courtID)
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Playing = normPlaying(court.Playing)
		if playingCount(court.Playing) >= 4 {
			return fmt.Errorf("court playing is full")
		}
		court.Queue = remove(court.Queue, playerID)
		if !contains(court.Playing, playerID) {
			for i := range court.Playing {
				if court.Playing[i] == "" {
					court.Playing[i] = playerID
					break
				}
			}
		}
		court.Status = model.CourtPlaying
		if playingCount(court.Playing) == 4 {
			court.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return nil
	})
}

// AdminAddToQueue puts a player into a court's queue (leader)
func AdminAddToQueue(ctx context.Context, sessionID, courtID, playerID string) error {
	pre, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if len(pre.Queue) >= 4 {
		return fmt.Errorf("排隊已滿")
	}
	if contains(pre.Queue, playerID) {
		return nil // already queued here
	}
	removeFromOtherCourts(ctx, sessionID, playerID, courtID)
	return updateCourt(ctx, sessionID, courtID, func(court *model.Court) error {
		court.Playing = normPlaying(court.Playing)
		if len(court.Queue) >= 4 {
			return fmt.Errorf("排隊已滿")
		}
		if contains(court.Queue, playerID) {
			return nil // already queued here
		}
		clearSlot(court.Playing, playerID) // 若原本在這場場上,先移除
		court.Queue = append(court.Queue, playerID)
		recomputeStatus(court) // 團主明確要排隊 → 不自動補上場
		return nil
	})
}

// toPlayingSlots returns a length-4 positional slice (empty slot → blank PlayerSlot)
func toPlayingSlots(playing []string, playerMap map[string]model.SessionPlayer) []model.PlayerSlot {
	p := normPlaying(playing)
	slots := make([]model.PlayerSlot, 4)
	for i, id := range p {
		if id == "" {
			continue // blank slot
		}
		pl := playerMap[id]
		slots[i] = model.PlayerSlot{
			PlayerID:    id,
			DisplayName: pl.DisplayName,
			Level:       pl.Level,
			Games:       pl.Games,
			AvatarURL:   pl.AvatarURL,
		}
	}
	return slots
}

func toSlots(ids []string, playerMap map[string]model.SessionPlayer) []model.PlayerSlot {
	slots := make([]model.PlayerSlot, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		p := playerMap[id]
		slots = append(slots, model.PlayerSlot{
			PlayerID:    id,
			DisplayName: p.DisplayName,
			Level:       p.Level,
			Games:       p.Games,
			AvatarURL:   p.AvatarURL,
		})
	}
	return slots
}

// fillFromQueue drains the queue into empty playing slots (缺幾補幾),
// then recomputes status + the 開打 clock. Keeps the invariant that the queue
// is only non-empty while the court is full.
// fillFromQueue drains the queue into empty slots and returns the promoted ids.
func fillFromQueue(court *model.Court) []string {
	court.Playing = normPlaying(court.Playing)
	promoted := []string{}
	for i := range court.Playing {
		if len(court.Queue) == 0 {
			break
		}
		if court.Playing[i] == "" {
			court.Playing[i] = court.Queue[0]
			promoted = append(promoted, court.Queue[0])
			court.Queue = court.Queue[1:]
		}
	}
	recomputeStatus(court)
	return promoted
}

// pushPromoted notifies each promoted player it's their turn — fanned out
// concurrently (goroutine per player) and waited on, so the up-to-4 pushes
// go out in parallel without the handler freezing before they finish.
func pushPromoted(ctx context.Context, court *model.Court, promoted []string) {
	if len(promoted) == 0 {
		return
	}
	name := court.Name
	if name == "" {
		name = fmt.Sprintf("場地 %d", courtNum(court.CourtID))
	}
	body := name + " · 快回來上場"
	var wg sync.WaitGroup
	for _, pid := range promoted {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			SendTurnPush(ctx, id, body)
		}(pid)
	}
	wg.Wait()
}

// recomputeStatus updates status + the 開打 clock from the current playing slots
// (no queue draining — used when we deliberately don't want to auto-fill).
func recomputeStatus(court *model.Court) {
	n := playingCount(court.Playing)
	if n == 0 {
		court.Status = model.CourtEmpty
		court.StartedAt = ""
		return
	}
	court.Status = model.CourtPlaying
	if n == 4 {
		if court.StartedAt == "" {
			court.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
	} else {
		court.StartedAt = "" // 還沒滿就不計時
	}
}

// normPlaying returns a length-4 positional slice ("" = empty slot)
func normPlaying(p []string) []string {
	out := make([]string, 4)
	for i := 0; i < 4 && i < len(p); i++ {
		out[i] = p[i]
	}
	return out
}

func playingCount(p []string) int {
	n := 0
	for _, s := range p {
		if s != "" {
			n++
		}
	}
	return n
}

func clearSlot(p []string, val string) {
	for i := range p {
		if p[i] == val {
			p[i] = ""
		}
	}
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val && val != "" {
			return true
		}
	}
	return false
}

func remove(slice []string, val string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			result = append(result, s)
		}
	}
	return result
}

func courtNum(courtID string) int {
	n := 0
	fmt.Sscanf(courtID, "court-%d", &n)
	return n
}
