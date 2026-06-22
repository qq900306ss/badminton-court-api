package service

import (
	"context"
	"fmt"
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
		cv := model.CourtView{
			CourtID:   c.CourtID,
			CourtNum:  courtNum(c.CourtID),
			Name:      c.Name,
			Status:    c.Status,
			Playing:   toSlots(c.Playing, playerMap),
			Queue:     toSlots(c.Queue, playerMap),
			StartedAt: c.StartedAt,
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

// JoinPlaying adds a player directly to playing if there's room (< 4)
func JoinPlaying(ctx context.Context, sessionID, courtID, playerID string) error {
	if err := validatePlayer(ctx, sessionID, playerID); err != nil {
		return err
	}
	if err := checkQueueOpen(ctx, sessionID); err != nil {
		return err
	}
	if cid, _ := playerInAnyCourt(ctx, sessionID, playerID); cid != "" {
		return fmt.Errorf("你已經在其他場地了,請先退出")
	}
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if len(court.Playing) >= 4 {
		return fmt.Errorf("court is full")
	}
	court.Playing = append(court.Playing, playerID)
	if len(court.Playing) > 0 {
		court.Status = model.CourtPlaying
	}
	// 開打計時只在「湊滿 4 人」那一刻才起算;1~3 人是湊人中,不計時
	if len(court.Playing) == 4 {
		court.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return repository.PutCourt(ctx, *court)
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
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if len(court.Queue) >= 4 {
		return fmt.Errorf("queue is full")
	}
	court.Queue = append(court.Queue, playerID)
	return repository.PutCourt(ctx, *court)
}

// LeaveQueue removes a player from a court's queue
func LeaveQueue(ctx context.Context, sessionID, courtID, playerID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	court.Queue = remove(court.Queue, playerID)
	return repository.PutCourt(ctx, *court)
}

// LeavePlaying lets a player leave a court that is still gathering (< 4).
// Once it's full (the game has started) only the leader can move people.
func LeavePlaying(ctx context.Context, sessionID, courtID, playerID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if len(court.Playing) >= 4 {
		return fmt.Errorf("比賽已開始,請找團主")
	}
	court.Playing = remove(court.Playing, playerID)
	if len(court.Playing) == 0 {
		court.Status = model.CourtEmpty
		court.StartedAt = ""
	}
	return repository.PutCourt(ctx, *court)
}

// creditFinishedGame gives everyone currently playing on the court +1 game and
// their minutes, and writes a GameLog. Shared by EndCourt and DeleteCourt.
func creditFinishedGame(ctx context.Context, sessionID string, court *model.Court) {
	if len(court.Playing) == 0 {
		return
	}
	now := time.Now().UTC()
	minutes := 0
	if court.StartedAt != "" {
		if started, perr := time.Parse(time.RFC3339, court.StartedAt); perr == nil {
			minutes = int(now.Sub(started).Minutes())
		}
	}
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
		EndedAtID:   now.Format(time.RFC3339) + "#" + uuid.New().String(),
		CourtNum:    courtNum(court.CourtID),
		PlayerNames: names,
		StartedAt:   court.StartedAt,
		EndedAt:     now.Format(time.RFC3339),
		Minutes:     minutes,
	})
}

// EndCourt rotates: playing → cleared, queue → playing.
// Everyone who was playing gets credited one game.
func EndCourt(ctx context.Context, sessionID, courtID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}

	creditFinishedGame(ctx, sessionID, court)

	court.Playing = court.Queue
	court.Queue = []string{}
	if len(court.Playing) == 0 {
		court.Status = model.CourtEmpty
		court.StartedAt = ""
	} else {
		court.Status = model.CourtPlaying
		// only start the clock if the next group is already full
		if len(court.Playing) == 4 {
			court.StartedAt = time.Now().UTC().Format(time.RFC3339)
		} else {
			court.StartedAt = ""
		}
	}
	return repository.PutCourt(ctx, *court)
}

// RenameCourt sets a court's custom name (leader)
func RenameCourt(ctx context.Context, sessionID, courtID, name string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	court.Name = name
	return repository.PutCourt(ctx, *court)
}

// RemoveCourt deletes a court (leader). Players currently playing are credited
// into the stats (as if the game ended); queued players are simply dropped.
func RemoveCourt(ctx context.Context, sessionID, courtID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	creditFinishedGame(ctx, sessionID, court) // 場上的人計入統計
	return repository.DeleteCourt(ctx, sessionID, courtID)
}

// KickPlayer removes a player from any state in a court (admin only)
func KickPlayer(ctx context.Context, sessionID, courtID, playerID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	court.Playing = remove(court.Playing, playerID)
	court.Queue = remove(court.Queue, playerID)
	if len(court.Playing) == 0 {
		court.Status = model.CourtEmpty
		court.StartedAt = ""
	}
	return repository.PutCourt(ctx, *court)
}

// AdminAddToPlaying force-adds a player to playing (admin only), bypassing queue
func AdminAddToPlaying(ctx context.Context, sessionID, courtID, playerID string) error {
	court, err := repository.GetCourt(ctx, sessionID, courtID)
	if err != nil {
		return err
	}
	if len(court.Playing) >= 4 {
		return fmt.Errorf("court playing is full")
	}
	court.Queue = remove(court.Queue, playerID)
	if !contains(court.Playing, playerID) {
		court.Playing = append(court.Playing, playerID)
	}
	court.Status = model.CourtPlaying
	if len(court.Playing) == 4 {
		court.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return repository.PutCourt(ctx, *court)
}

func toSlots(ids []string, playerMap map[string]model.SessionPlayer) []model.PlayerSlot {
	slots := make([]model.PlayerSlot, 0, len(ids))
	for _, id := range ids {
		p := playerMap[id]
		slots = append(slots, model.PlayerSlot{
			PlayerID:    id,
			DisplayName: p.DisplayName,
			Level:       p.Level,
			Games:       p.Games,
		})
	}
	return slots
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func remove(slice []string, val string) []string {
	result := slice[:0]
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
