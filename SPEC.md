# 規格文件

## 背景

Immersive Translate 支援自訂翻譯 API。此專案目標是用 Go 建立一個輕量 HTTP 服務，將 Immersive Translate 的自訂 API 請求轉接到 Google Translate `translateHtml` 端點。

## 目標

- 對外提供 Immersive Translate 相容 API。
- 對內呼叫 Google Translate `translateHtml`。
- 支援批次文字翻譯。
- 使用環境變數管理 Google API key。
- 提供健康檢查端點，方便部署平台探測。

## 非目標

- 不提供前端頁面。
- 不提供資料庫或持久化儲存。
- 不內建使用者帳號、權限或計費。
- 不在版本控制中保存真實 API key。
- 不保證 Google 非公開端點的長期相容性。

## 外部 API 契約

服務需支援 Immersive Translate 自訂 API 規格。

### 請求

- 方法：`POST`
- 路徑：`/translate`
- Content-Type：`application/json`
- Body 欄位：
  - `source_lang`：來源語言代碼，可為 `auto`。
  - `target_lang`：目標語言代碼，必填。
  - `text_list`：待翻譯文字陣列，至少一筆。

### 回應

- Content-Type：`application/json`
- Body 欄位：
  - `translations`：翻譯結果陣列。
  - `translations[].detected_source_lang`：偵測或使用的來源語言代碼。
  - `translations[].text`：翻譯後文字。

## Google 上游 API

### 端點

```text
https://translate-pa.googleapis.com/v1/translateHtml
```

### Headers

```http
Content-Type: application/json+protobuf
X-Goog-API-Key: <GOOGLE_TRANSLATE_API_KEY>
```

### Body

```json
[
  [["Hello world"], "en", "zh-TW"],
  "wt_lib"
]
```

第一層陣列的第一個元素包含文字陣列、來源語言與目標語言，第二個元素固定為 Google web translate client `wt_lib`。

### 預期回應

Google 回應為陣列格式，第一個元素應為翻譯結果陣列。服務會把結果映射為 Immersive Translate 的 `translations`。

## 設定

| 環境變數 | 必填 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `GOOGLE_TRANSLATE_API_KEY` | 是 | 無 | Google Translate API key。 |
| `PORT` | 否 | `8080` | HTTP 服務監聽連接埠。 |
| `GOOGLE_TRANSLATE_URL` | 否 | `https://translate-pa.googleapis.com/v1/translateHtml` | Google 上游 URL，主要供測試或除錯覆寫。 |

本機開發可使用 `.env`，正式環境建議使用平台提供的 secret 或環境變數功能。

## 錯誤處理

- 非 `POST /translate` 請求回傳 `405 Method Not Allowed`。
- JSON 格式錯誤回傳 `400 Bad Request`。
- 缺少必要欄位或 `text_list` 為空回傳 `400 Bad Request`。
- 未設定 `GOOGLE_TRANSLATE_API_KEY` 時回傳 `500 Internal Server Error`。
- Google 上游失敗或格式不符時回傳 `502 Bad Gateway`。

## 測試策略

- 使用 `httptest` 建立 mock Google 上游，不在測試中呼叫真實 Google API。
- 驗證 Immersive Translate 請求格式解析。
- 驗證 Google 上游 request body 與 headers。
- 驗證成功回應映射。
- 驗證常見錯誤路徑。

## 安全性

- 真實 `.env` 檔案不可提交。
- API key 不應出現在 log、錯誤訊息、文件或測試快照中。
- 若部署於公開網路，建議放在反向代理後方並加上存取限制。
