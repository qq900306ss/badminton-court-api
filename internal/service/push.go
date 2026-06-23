package service

import (
	"context"
	"encoding/json"
	"os"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// SendTurnPush notifies a player it's their turn (best-effort; cleans up dead subs).
func SendTurnPush(ctx context.Context, playerID, body string) {
	pub := os.Getenv("VAPID_PUBLIC_KEY")
	priv := os.Getenv("VAPID_PRIVATE_KEY")
	if pub == "" || priv == "" {
		return
	}
	sub, err := repository.GetPushSub(ctx, playerID)
	if err != nil || sub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"title": "🏸 輪到你上場了!",
		"body":  body,
	})
	subject := os.Getenv("VAPID_SUBJECT")
	if subject == "" {
		subject = "mailto:admin@example.com"
	}

	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	resp, err := webpush.SendNotificationWithContext(cctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber:      subject,
		VAPIDPublicKey:  pub,
		VAPIDPrivateKey: priv,
		TTL:             60,
	})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		_ = repository.DeletePushSub(ctx, playerID) // subscription gone — clean up
	}
}
