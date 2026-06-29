# badminton-court-api

羽球場地管理系統的後端 API。團主開團、開球場,臨打人(散客)掃 QR 進場、自己上下場/排隊,全程**即時同步**。

**Go + Gin**,跑在 **Fly.io 常駐伺服器**(東京 `nrt`,刻意單台機器),資料存 **DynamoDB**(`ap-northeast-1`),頭像照片存 **S3**。

## 線上網址

| 服務 | 網址 |
|------|------|
| **API** | https://badminton-court-api.fly.dev |
| 臨打人前端 (booking) | https://d2mg2bpjvlg672.cloudfront.net |
| 團主後台 (admin) | https://d1r9u0ja59y4rv.cloudfront.net |

## 架構

```
玩家手機 / 團主後台
      │  HTTPS + WebSocket
      ▼
 Fly.io 單台機器 (cmd/server, 常駐)
   ├─ Gin HTTP API
   ├─ in-process WebSocket Hub（一個 session 一個房間）
   └─ 背景排程:過期場次自動關團
      │
      ├─ DynamoDB (ap-northeast-1)  ← 所有資料
      └─ S3  ← 自訂上傳的頭像照片
```

- **為什麼是單台機器**:即時同步用「行程內」WebSocket Hub 廣播,所有連線必須連到同一台,因此 `auto_stop_machines = off`、`min_machines_running = 1`。
- **即時怎麼運作**:任何成功的非-GET `/sessions/:id/*` 請求,經 `BroadcastOnChange` middleware 對該 session 房間推一個極小的 `{"t":"changed"}`;前端收到就重抓。被踢/被改名則推 targeted 事件。前端另有 30 秒慢速輪詢當斷線後備。
- **`cmd/server`** = 常駐入口(正式環境)。**`cmd/lambda`** = 舊的 AWS Lambda 入口,保留當退路,已不使用。

## 功能總覽

- **強制登入**:臨打人用 Google / LINE 登入、團主用 Google 登入,全程帶 JWT(已無 X-Player-ID)。
- **開團 / 球場**:多球場、湊 4 人開打計時、排隊、樂觀鎖(`Court.Version` 條件式寫入)防搶位。
- **團主代排**:現場排點板(代沒手機的人上下場),規則與玩家端一致。
- **投票結束場地**:場上湊滿 4 人後,3 人投票即自動結束換下一組;結果寫入操作紀錄。
- **還原結束**:10 分鐘內可復原誤按的「結束場地」,還原場上/排隊/計時並退回場數。
- **家人共用手機**:一個帳號可帶多位家人(子身份),團主審核後才能被排點;手機可代操作自家成員。
- **臨打費標記**:團主標記誰已繳場地費。
- **團主操作紀錄**:踢人/移人/改名/結束等動作留痕,DynamoDB **TTL 90 天**自動清。
- **意見回饋**:玩家與團主皆可送出,超級管理員統一檢視。
- **縣市發現目錄**:大廳可依縣市/區瀏覽進行中的公開團。
- **頭像**:emoji 預設 / Google·LINE 大頭貼 / 自訂上傳(S3 presigned PUT)。
- **Web Push**:輪到你上場、被移除時推播。
- **自動關團**:背景排程每 10 分鐘掃過期場次,結束時間後 2 小時自動關。
- **防濫用**:全域 per-IP 限流(密碼驗證另有嚴格限流)、64KB body 上限。

## 認證模型

| 角色 | 登入方式 | JWT 內容 | Middleware |
|------|----------|----------|------------|
| 臨打人 (player) | Google / LINE OAuth | `role:player` + `player_id` | `RequirePlayer` |
| 團主 (leader) | Google OAuth | `org_id` + `email` | `RequireAuth` |
| 超級管理員 | 同團主,email 在 `SUPER_ADMIN_EMAILS` | 同上 + 角色判定 | `RequireSuperAdmin` |

## API 路由

**公開(免登入)**
| 方法 | 路徑 | 說明 |
|---|---|---|
| POST | `/api/auth/google` | 團主 Google 登入 |
| POST | `/api/auth/player/{google,line}` | 玩家登入 |
| GET | `/api/push/vapid` | 取 Web Push 公鑰 |
| GET | `/api/sessions/open` | 大廳:進行中的公開團 |
| GET | `/api/sessions/:id` | 場次即時狀態 |
| GET | `/api/sessions/:id/players` | 本場人員 |
| POST | `/api/sessions/:id/verify-password` | 驗證進場密碼(嚴格限流) |
| GET | `/api/sessions/:id/ws` | WebSocket 即時推播 |

**玩家(RequirePlayer)**
`GET/PUT /api/players/me`、`POST /api/players/me/avatar-upload-url`、`POST /api/sessions/:id/join`、`POST /api/sessions/:id/push-subscribe`、`POST/DELETE /api/sessions/:id/family[/:playerId]`、`POST /api/feedback`、球場動作 `POST /api/sessions/:id/courts/:courtId/{join-playing,join-queue,leave-queue,leave-playing,vote-end}`(可帶 `as_player` 代家人)。

**團主(RequireAuth)**
`GET /api/auth/me`、`GET /api/my/sessions`、`POST /api/my/feedback`、`GET /api/sessions/:id/{games,action-logs}`、`POST /api/sessions`、人員 `players/:id/{level,name,paid,approve}` + `DELETE`、`close`、`GET/PUT password`、`PUT times`、球場 `courts...`(新增/改名/刪除/結束/還原/踢人/代排上下場)。

**超級管理員(RequireSuperAdmin)**
`/api/admin/{orgs,impersonate,sessions,feedback}`。

## DynamoDB 資料表(前綴預設 `badminton-`)

`sessions`(+status GSI)、`session-players`、`courts`、`game-logs`、`players`(+provider GSI)、`push-subscriptions`、`session-history`、`orgs`、`action-logs`(TTL `expires_at`)、`feedback`。表不存在會在啟動時自動建立(best-effort)。

## 本機開發

```bash
go run ./cmd/server     # http://localhost:8080
go build ./... && go vet ./...
go test ./...
```

需要可連到 AWS 的金鑰(DynamoDB/S3)與下列環境變數。

## 環境變數

| 變數 | 用途 |
|------|------|
| `JWT_SECRET` | 簽發 / 驗證 JWT |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth |
| `GOOGLE_REDIRECT_URI` / `GOOGLE_PLAYER_REDIRECT_URI` | 團主 / 玩家 Google 轉址 |
| `LINE_CHANNEL_ID` / `LINE_CHANNEL_SECRET` / `LINE_REDIRECT_URI` | LINE 登入 |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` / `VAPID_SUBJECT` | Web Push |
| `SUPER_ADMIN_EMAILS` | 超級管理員 email(逗號分隔) |
| `ALLOWED_ORIGINS` | CORS 白名單 |
| `AWS_REGION`(+ AWS 金鑰) | DynamoDB / S3 連線 |
| `TABLE_PREFIX` | DynamoDB 表前綴(預設 `badminton`) |
| `PORT` | 監聽埠(預設 8080) |

正式環境的值放在 `fly secrets`(不進 git)。

## 部署

push 到 `main` → Fly.io 透過 GitHub 整合**自動 build + deploy**(`Dockerfile` + `fly.toml`,東京、單台、常駐)。

```bash
flyctl deploy            # 手動部署(備援)
flyctl secrets set KEY=value
flyctl logs
```

機器規格:`shared-cpu-1x` / 256MB,always-on,月費約 US$2。完整維運說明見 `../DEPLOY.md`。
