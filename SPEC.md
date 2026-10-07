# 規格文件

## 背景

Immersive Translate 支援自訂翻譯 API。此專案目標是用 Go 建立一個輕量 HTTP 服務，將 Immersive Translate 的自訂 API 請求轉接到 Google Translate `translateHtml` 端點。

## 目標

- 對外提供 Immersive Translate 相容 API。
- 對內呼叫 Google Translate `translateHtml`。
- 支援批次文字翻譯。
- 限制單次請求 body 最大 1 MiB，降低異常或惡意請求造成的資源耗盡風險。
- 使用環境變數管理 Google API key。
- 提供健康檢查端點，方便部署平台探測。
- 首次啟動若 `.env` 不存在，使用 `.env.example` 範本自動建立。
- 提供 PowerShell 編譯腳本，預設輸出目前環境執行檔，也可依參數輸出 Windows、Linux、macOS 的 x86 與 ARM 執行檔到 `dist` 目錄。
- 提供 GitHub Actions，在推送 `v*` tag 時測試、全平台編譯並上傳 GitHub Release。
- 執行檔提供服務管理指令，可直接安裝、啟動、停止、重新啟動、查詢狀態與移除系統服務。

## 非目標

- 不提供前端頁面。
- 不提供資料庫或持久化儲存。
- 不內建使用者帳號、權限或計費。
- 不在版本控制中保存私人 API key；既有公開預設金鑰為明確例外。
- 不保證 Google 非公開端點的長期相容性。

## 外部 API 契約

服務需支援 Immersive Translate 自訂 API 規格。

### 請求

- 方法：`POST`
- 路徑：`/translate`
- 建議 Content-Type：`application/json`，不強制檢查。
- 請求欄位：
  - `source_lang`：來源語言代碼，可為 `auto`。
  - `target_lang`：目標語言代碼，必填。
  - `text_list`：待翻譯文字陣列，至少一筆。

### 回應

- Content-Type：`application/json; charset=utf-8`
- 回應欄位：
  - `translations`：翻譯結果陣列。
  - `translations[].detected_source_lang`：使用的來源語言代碼，`auto` 會原樣回傳，不代表已偵測語言。
  - `translations[].text`：翻譯後文字。

## Google 上游 API

### 端點

```text
https://translate-pa.googleapis.com/v1/translateHtml
```

### 請求標頭

```http
Content-Type: application/json+protobuf
X-Goog-API-Key: <GOOGLE_TRANSLATE_API_KEY>
```

### 請求內容

```json
[
  [["Hello world"], "en", "zh-TW"],
  "wt_lib"
]
```

第一層陣列的第一個元素包含文字陣列、來源語言與目標語言，第二個元素固定為 Google web translate client `wt_lib`。

### 預期回應

Google 回應為陣列格式，第一個元素應為翻譯結果陣列。服務會把結果映射為 Immersive Translate 的 `translations`。

Google 回應最大 1 MiB，讀取額外一個位元組以識別超量，不接受單純截斷後的結果。第一個元素內的翻譯必須全為字串，筆數必須與輸入一致，維持原始順序。上游逾時為 30 秒；不新增重試或快取。

## 設定

| 環境變數 | 必填 | 預設值 | 說明 |
| --- | --- | --- | --- |
| `GOOGLE_TRANSLATE_URL` | 否 | `https://translate-pa.googleapis.com/v1/translateHtml` | Google 上游 URL，主要供測試或除錯覆寫。 |
| `GOOGLE_TRANSLATE_API_KEY` | 否 | 程式內建公開 key | Google Translate API key，可用環境變數覆寫。 |
| `PORT` | 否 | `9009` | HTTP 服務監聽連接埠。 |

本機開發可使用 `.env`。首次啟動若 `.env` 不存在，服務會優先複製外部 `.env.example`；若執行環境沒有 `.env.example`，則使用編譯時嵌入的範本內容。若 `.env` 與系統環境變數都沒有設定 `GOOGLE_TRANSLATE_API_KEY`，服務會使用程式內建公開 key。正式環境可直接使用平台提供的環境變數功能覆寫設定。

- 設定優先順序為非空系統環境變數、檔案、預設值；選定值去除前後空白後若為空，改用預設值。
- 預設連接埠改為 `9009` 不會遷移既有 `PORT` 設定，也不覆寫既有 `.env`。
- 解析 `.env` 不呼叫 `os.Setenv`；重複鍵維持第一個非空值優先。
- 支援 UTF-8 BOM、空行、註解行、簡單單引號及雙引號；忽略不含等號的行，不展開變數或解析行尾註解。單行受標準掃描器約 64 KiB 限制。
- 使用排他建立方式，避免同時啟動覆寫既有 `.env`；啟動工作目錄須能建立缺少的檔案。
- `PORT` 必須為 `1` 至 `65535`；上游網址限 HTTP／HTTPS，須有主機，不含登入資訊與片段。錯誤訊息不回顯設定值。

## 程式架構

維持單一 `main` 套件，依責任拆檔：入口與指令、設定與環境檔案、HTTP 伺服器與系統服務、API 處理器、Google 翻譯器。保留 `go run .` 與根目錄建置方式，不新增相依套件。

伺服器建構只接收已驗證的設定，不讀寫檔案或修改環境。翻譯器與 HTTP 用戶端可替換，指令派送可使用測試用的服務物件。僅入口決定程序退出，底層透過錯誤或通道通知。

## 服務管理

執行檔支援以下服務管理指令：

| 指令 | 說明 |
| --- | --- |
| `run` | 前景執行服務。 |
| `install` | 安裝系統服務。 |
| `start` | 啟動系統服務。 |
| `stop` | 停止系統服務。 |
| `restart` | 重新啟動系統服務。 |
| `status` | 顯示服務狀態，輸出 `running`、`stopped` 或 `unknown`。 |
| `uninstall` | 移除系統服務。 |

安裝服務時，程式會記錄執行 `install` 當下的工作目錄，服務啟動後以該目錄讀取或建立 `.env`。Windows 安裝或移除服務通常需要系統管理員權限；Linux 與 macOS 依服務管理器設定可能需要 `sudo`。

- 保留內部 `service-run --workdir 路徑` 機制，因 Windows 不使用服務套件的工作目錄設定。
- 前景與 `help` 不建立服務物件；無指令時仍依互動模式選擇前景或服務。
- 指令不區分大小寫；拒絕多餘參數及重複工作目錄參數，完整驗證後才切換目錄。
- 連接埠同步綁定成功後，才回報啟動成功；執行中異常會傳回入口。
- 停止可重複或同時呼叫；共用優雅關閉，最多等待 10 秒，逾時強制關閉連線。
- 前景支援 Ctrl+C 與終止訊號；使用方式錯誤退出碼為 `2`，其他執行錯誤為 `1`。
- 維持所有網路介面監聽，適用本機與可信任內網。

## 編譯

使用 Go 1.25 以上與 `build.ps1` 可將服務編譯為單一執行檔。預設只編譯目前 Go 環境，可透過平台參數選擇其他目標。

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

本專案的 macOS 建置目標為 `amd64` 與 `arm64`。

腳本會設定 `CGO_ENABLED=0`，降低執行檔對外部動態連結函式庫的依賴。

腳本以專案目錄建置並還原原本的環境變數。清理僅限專案內輸出子目錄，拒絕根目錄、上層、版本庫資料與連結；自訂執行檔名稱不可包含路徑。

## 持續整合

`.github/workflows/ci.yml` 在一般分支推送與拉取請求執行：

- Windows／Linux 的格式檢查、單元與本機整合測試、`go vet`、建置安全測試。
- Linux 的 `go test -race`，需要 C 編譯器。
- Linux 主機上的全部 12 種交叉編譯。

## 自動發佈

專案提供 `.github/workflows/release.yml`。當 push 的 Git tag 符合 `v*` 時，workflow 需執行以下流程：

- 使用 `actions/setup-go` 依 `go.mod` 設定 Go 版本。
- 執行測試、`go vet`、Linux 競態檢查與建置安全測試。
- 使用 PowerShell 執行 `./build.ps1 -Clean -All`，輸出全部支援平台的執行檔到 `dist` 目錄。
- 使用 GitHub Release action 建立或更新對應 tag 的 Release。
- 將 `dist/*` 內所有檔案上傳為 Release assets。

workflow 需要 `contents: write` 權限，才能建立 Release 與上傳附件。

## 錯誤處理

- `/translate` 使用非 POST 方法時回傳 `405 Method Not Allowed`。
- `/healthz` 限 GET；兩個端點的 `405` 回應包含 `Allow` 標頭。
- JSON 格式錯誤、未知欄位、尾端額外 JSON 或非空白內容回傳 `400 Bad Request`。
- 缺少必要欄位或 `text_list` 為空回傳 `400 Bad Request`。
- 請求 body 超過 1 MiB 時回傳 `413 Payload Too Large`。
- 若翻譯器內部 API key 設定為空時回傳 `500 Internal Server Error`。
- Google 上游失敗或格式不符時回傳 `502 Bad Gateway`。
- Google 回應超量、翻譯筆數不符、取消或逾時仍回傳 `502`，不變更既有錯誤碼契約。

## 測試策略

- 使用 `httptest` 建立 mock Google 上游，不在測試中呼叫真實 Google API。
- 驗證 Immersive Translate 請求格式解析。
- 驗證 Google 上游 request body 與 headers。
- 驗證成功回應映射。
- 驗證常見錯誤路徑。
- 驗證大小限制前後邊界、尾端內容、上游筆數、取消、逾時、讀取錯誤與敏感資訊不外洩。
- 驗證設定優先順序、不修改環境、環境檔案並行建立、指令驗證與工作目錄。
- 驗證連接埠占用、執行中失敗、請求排空、逾時強制關閉與重複停止；不自動安裝真正系統服務。

## 安全性

- 真實 `.env` 檔案不可提交。
- 私人 API key 不應出現在紀錄、錯誤訊息、文件或測試快照中；公開預設金鑰可在範本與使用說明中呈現。
- 上游錯誤僅紀錄固定分類與 HTTP 狀態碼，不紀錄上游回應內容、網址、請求文字或譯文。
- HTTP server 設定讀取 header、讀取 body、寫入回應與 idle 連線逾時，避免慢速連線長時間占用資源。
- 若部署於公開網路，建議放在反向代理後方並加上存取限制。
- Windows 的檔案權限不由 Unix `0600` 模式保證，須依部署環境另行設定存取控制。
