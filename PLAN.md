# 實作計畫

## TODO

- [x] 建立 Go module。
- [x] 建立 HTTP server 入口。
- [x] 加入 `.env` 載入邏輯，允許不存在 `.env` 時改用系統環境變數。
- [x] 首次啟動若 `.env` 不存在，自動由 `.env.example` 建立。
- [x] 實作 `GET /healthz` 健康檢查。
- [x] 實作 `POST /translate` 請求解析與驗證。
- [x] 實作 Google Translate `translateHtml` client。
- [x] 將 Google 回應映射為 Immersive Translate 回應格式。
- [x] 加入一致的 JSON 錯誤回應格式。
- [x] 加入 HTTP server 逾時與 1 MiB 請求 body 限制，降低資源耗盡風險。
- [x] 撰寫 handler 與 Google client 測試。
- [x] 撰寫 `.env` 自動建立測試。
- [x] 新增 `build.ps1` 編譯腳本。
- [x] 支援 Windows、Linux、macOS 的 x86 與 ARM 編譯輸出。
- [x] 調整 `build.ps1` 預設只編譯目前環境，並用平台參數選擇輸出目標。
- [x] 將編譯輸出目錄 `dist` 加入忽略。
- [x] 執行 `gofmt`。
- [x] 執行 `go test ./...`。
- [x] 視實作結果更新 `README.md`、`SPEC.md`、`API_SPEC.md` 與 `openapi.yaml`。

## 提交策略

- 文件先行提交，先固定規格與實作 TODO。
- 程式碼完成後另行提交，避免文件與程式碼混在同一個提交中。
- 若程式碼實作後導致文件需要修正，文件更新再獨立提交。
