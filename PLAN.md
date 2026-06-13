# 實作計畫

## TODO

- [ ] 建立 Go module。
- [ ] 建立 HTTP server 入口。
- [ ] 加入 `.env` 載入邏輯，允許不存在 `.env` 時改用系統環境變數。
- [ ] 實作 `GET /healthz` 健康檢查。
- [ ] 實作 `POST /translate` 請求解析與驗證。
- [ ] 實作 Google Translate `translateHtml` client。
- [ ] 將 Google 回應映射為 Immersive Translate 回應格式。
- [ ] 加入一致的 JSON 錯誤回應格式。
- [ ] 撰寫 handler 與 Google client 測試。
- [ ] 執行 `gofmt`。
- [ ] 執行 `go test ./...`。
- [ ] 視實作結果更新 `README.md`、`SPEC.md`、`API_SPEC.md` 與 `openapi.yaml`。

## 提交策略

- 文件先行提交，先固定規格與實作 TODO。
- 程式碼完成後另行提交，避免文件與程式碼混在同一個提交中。
- 若程式碼實作後導致文件需要修正，文件更新再獨立提交。
