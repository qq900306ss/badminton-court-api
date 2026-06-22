# badminton-court-api

羽球場地管理系統的後端 API(Go + Gin,跑在 AWS Lambda + DynamoDB)。

## 線上網址

| 服務 | 網址 |
|------|------|
| **API** | https://pp2p4ln2cogxt4mi5f2wl3rqi40vskvs.lambda-url.ap-northeast-1.on.aws |
| 臨打人前端 (booking) | https://d2mg2bpjvlg672.cloudfront.net |
| 團主後台 (admin) | https://d1r9u0ja59y4rv.cloudfront.net |

## 本機開發

```bash
go run ./cmd/server   # http://localhost:8080
```

## 部署

push 到 `main` → GitHub Actions 自動 build(linux/arm64)並更新 Lambda。
完整部署說明見 `../DEPLOY.md`。
