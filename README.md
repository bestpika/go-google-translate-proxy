# go-google-translate-proxy

將 Google Translate `translateHtml` 端點包裝成 Immersive Translate 自訂翻譯服務可使用的 HTTP API。

## 功能目標

- 提供 Immersive Translate 自訂 API 相容的 `POST /translate` 端點。
- 接收 `source_lang`、`target_lang`、`text_list`，批次轉送至 Google Translate。
- 回傳 `translations` 陣列，保持與 Immersive Translate 文件一致。
- 請求與 Google 回應各限制 1 MiB，完整驗證 JSON 與翻譯筆數。
- 保留內建公開 Google 金鑰，私人金鑰可透過環境變數覆寫且不可提交。
- 提供 `GET /healthz` 作為健康檢查。
- 首次啟動若沒有 `.env`，會使用 `.env.example` 範本自動建立。
- 提供 `build.ps1` 編譯目前環境，或依參數編譯 Windows、Linux、macOS 的 x86 與 ARM 執行檔到 `dist` 目錄。
- 提供 GitHub Actions，在推送 `v*` tag 時全平台編譯並上傳 GitHub Release。
- 執行檔可直接安裝、啟動、停止或移除作業系統服務。
- 前景與系統服務共用啟停流程，停止時等待處理中的請求，最多 10 秒。
- 一般推送與拉取請求會執行 Windows／Linux 測試、靜態檢查、Linux 競態檢查與跨平台建置。

## 設定

首次啟動若沒有 `.env`，服務會使用 `.env.example` 範本自動建立。`.env.example` 已提供預設公開 Google Translate API key，也可以自行改用其他 key。

若 `.env` 存在但沒有設定 `GOOGLE_TRANSLATE_API_KEY`，且系統環境變數也沒有設定，服務會使用程式內建的預設公開 API key。

```env
GOOGLE_TRANSLATE_URL=https://translate-pa.googleapis.com/v1/translateHtml
GOOGLE_TRANSLATE_API_KEY=AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520
PORT=8080
```

`.env` 只供本機覆寫設定使用，已由 `.gitignore` 忽略。若改用私人金鑰，請勿提交。

設定優先順序為非空的系統環境變數、`.env`、內建預設值。值會去除前後空白；只有空白的系統值仍優先於檔案，但去除空白後會回到內建預設值。讀取 `.env` 不會修改程序的環境變數。

`PORT` 必須是 `1` 至 `65535` 的整數；`GOOGLE_TRANSLATE_URL` 必須是具有主機名稱的 HTTP 或 HTTPS 網址，不可包含登入資訊或片段。錯誤設定會在開始監聽前被拒絕。

部署環境可直接使用系統環境變數覆寫設定，但啟動仍會自動建立缺少的 `.env`，因此工作目錄必須可寫。既有 `.env` 不會被覆寫。`.env` 支援 UTF-8、UTF-8 BOM、註解行與簡單引號，不支援變數展開或行尾註解。

## 啟動方式

```powershell
go run .
```

預設在所有網路介面監聽連接埠 `8080`，可透過 `PORT` 覆寫。本機可使用 `localhost`，內網裝置可使用這台電腦的 IP；請使用防火牆限制可信任的來源。

執行檔也可明確使用 `run` 前景啟動：

```powershell
.\go-google-translate-proxy.exe run
```

前景模式收到 Ctrl+C 或作業系統的終止訊號後會停止接受新連線，等待既有請求完成；超過 10 秒則關閉剩餘連線。`help` 與前景模式不會建立系統服務物件。

## 服務安裝

編譯後的執行檔可自行安裝成系統服務。請在放置 `.env` 的目錄執行 `install`，服務會記住該目錄作為工作目錄。

```powershell
.\go-google-translate-proxy.exe install
.\go-google-translate-proxy.exe start
.\go-google-translate-proxy.exe status
```

可用指令如下：

| 指令 | 說明 |
| --- | --- |
| `run` | 前景執行服務。 |
| `install` | 安裝系統服務。 |
| `start` | 啟動系統服務。 |
| `stop` | 停止系統服務。 |
| `restart` | 重新啟動系統服務。 |
| `status` | 顯示服務狀態。 |
| `uninstall` | 移除系統服務。 |

Windows 安裝或移除服務通常需要系統管理員權限；Linux 與 macOS 依服務管理器設定可能需要 `sudo`。

## 編譯

需要 Go 1.25 以上。

```powershell
.\build.ps1
```

預設只會編譯目前 Go 環境，結果會輸出到 `dist` 目錄。

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

若要先清空舊的 `dist` 目錄再編譯：

```powershell
.\build.ps1 -Clean -All
```

`-OutputDir` 可指定其他輸出目錄；`-Clean` 僅允許專案內的輸出子目錄，拒絕根目錄、上層目錄、`.git`、`.github` 與包含連結的路徑。`-BinaryName` 必須是檔名，不可包含路徑。腳本可從其他工作目錄呼叫，並會還原建置用的環境變數。

## 發佈

推送 `v` 開頭的 tag 時，GitHub Actions 會執行測試、全平台編譯，並將 `dist` 目錄內所有檔案上傳到同一個 tag 的 GitHub Release。

```powershell
git tag v1.0.0
git push origin v1.0.0
```

發佈工作流程會執行：

```powershell
go test -mod=readonly -count=1 ./...
go vet -mod=readonly ./...
.\build_test.ps1
.\build.ps1 -Clean -All
```

發佈工作流程另執行 Linux 競態檢查，成功後才上傳產物。

## 測試

```powershell
go test ./...
go vet ./...
.\build_test.ps1
```

測試使用本機模擬上游，不呼叫真實 Google，也不安裝系統服務。Linux CI 另執行 `go test -race ./...`；本機執行競態檢查需要可用的 C 編譯器。Go 原始碼使用 `gofmt` 與 LF 換行。

## 程式結構

維持單一 Go 套件與根目錄入口，不引入新框架：

| 檔案 | 責任 |
| --- | --- |
| `main.go`、`command.go` | 入口、退出碼、指令解析與派送。 |
| `config.go`、`dotenv.go` | 設定驗證、環境檔案解析與建立。 |
| `server.go`、`service.go` | 監聽、前景與系統服務生命週期。 |
| `http_handler.go` | API 契約、輸入驗證與錯誤回應。 |
| `google_translator.go` | 上游請求、限制與解析。 |

對應的 `_test.go` 驗證各責任；`build_test.ps1` 驗證建置路徑安全。

## Immersive Translate 設定

在 Immersive Translate 自訂 API 中設定服務網址：

```text
http://localhost:8080/translate
```

若服務部署在遠端，請改成自己的 HTTPS 網址。

服務沒有認證或流量限制，不建議直接暴露到公開網路。Google 端點與預設公開金鑰並非本專案可保證的穩定服務，可能失效或遭限流。

## 請求範例

```http
POST /translate HTTP/1.1
Content-Type: application/json

{
  "source_lang": "en",
  "target_lang": "zh-TW",
  "text_list": ["Hello world"]
}
```

## 回應範例

```json
{
  "translations": [
    {
      "detected_source_lang": "en",
      "text": "你好，世界"
    }
  ]
}
```

## 相關文件

- `SPEC.md`：產品與技術規格。
- `API_SPEC.md`：HTTP API 細節。
- `openapi.yaml`：OpenAPI 規格。
- `PLAN.md`：實作 TODO。
- `MEMORY.md`：開發交接與驗證限制。
- `CHANGELOG.md`：更新紀錄。

## 授權

本專案採用 MIT License。詳見 `LICENSE`。

此授權僅涵蓋本專案程式碼與文件，不代表授權使用 Google Translate API、API key 或 Google 服務本身。
