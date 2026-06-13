# 規格文件

## 背景

Immersive Translate 支援自訂翻譯 API。此專案目標是用 Go 建立一個輕量 HTTP 服務，將 Immersive Translate 的自訂 API 請求轉接到 Google Translate `translateHtml` 端點。

## 目標

- 對外提供 Immersive Translate 相容 API。
- 對內呼叫 Google Translate `translateHtml`。
- 支援批次文字翻譯。
- 使用環境變數管理 Google API key。
- 提供健康檢查端點，方便部署平台探測。
- 首次啟動若 `.env` 不存在，使用 `.env.example` 範本自動建立。
- 提供 PowerShell 編譯腳本，預設輸出目前環境執行檔，也可依參數輸出 Windows、Linux、macOS 的 x86 與 ARM 執行檔到 `dist` 目錄。

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
| `GOOGLE_TRANSLATE_URL` | 否 | `https://translate-pa.googleapis.com/v1/translateHtml` | Google 上游 URL，主要供測試或除錯覆寫。 |
| `GOOGLE_TRANSLATE_API_KEY` | 是 | `.env.example` 範本值 | Google Translate API key。 |
| `PORT` | 否 | `8080` | HTTP 服務監聽連接埠。 |

本機開發可使用 `.env`。首次啟動若 `.env` 不存在，服務會優先複製外部 `.env.example`；若執行環境沒有 `.env.example`，則使用編譯時嵌入的範本內容。正式環境可直接使用平台提供的環境變數功能覆寫設定。

## 編譯

使用 `build.ps1` 可將服務編譯為單一執行檔。預設只編譯目前 Go 環境，可透過平台參數選擇其他目標。

```powershell
.\build.ps1
```

| 指令 | 說明 |
| --- | --- |
| `.\build.ps1` | 編譯目前環境。 |
| `.\build.ps1 -Windows` | 編譯所有 Windows 目標。 |
| `.\build.ps1 -Linux` | 編譯所有 Linux 目標。 |
| `.\build.ps1 -MacOS` | 編譯所有 macOS 目標。 |
| `.\build.ps1 -Windows -Linux` | 同時編譯 Windows 與 Linux 目標。 |
| `.\build.ps1 -All` | 編譯全部支援目標。 |

支援目標如下：

| 檔案 | 平台 |
| --- | --- |
| `go-google-translate-proxy-windows-386.exe` | Windows x86 32-bit |
| `go-google-translate-proxy-windows-amd64.exe` | Windows x86 64-bit |
| `go-google-translate-proxy-windows-armv7.exe` | Windows ARM 32-bit |
| `go-google-translate-proxy-windows-arm64.exe` | Windows ARM 64-bit |
| `go-google-translate-proxy-linux-386` | Linux x86 32-bit |
| `go-google-translate-proxy-linux-amd64` | Linux x86 64-bit |
| `go-google-translate-proxy-linux-armv5` | Linux ARMv5 32-bit |
| `go-google-translate-proxy-linux-armv6` | Linux ARMv6 32-bit |
| `go-google-translate-proxy-linux-armv7` | Linux ARMv7 32-bit |
| `go-google-translate-proxy-linux-arm64` | Linux ARM 64-bit |
| `go-google-translate-proxy-macos-amd64` | macOS x86 64-bit |
| `go-google-translate-proxy-macos-arm64` | macOS ARM 64-bit |

macOS 目前在 Go 1.21 僅支援 `amd64` 與 `arm64`。

腳本會設定 `CGO_ENABLED=0`，降低執行檔對外部動態連結函式庫的依賴。

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
