# go-google-translate-proxy

將 Google Translate `translateHtml` 端點包裝成 Immersive Translate 自訂翻譯服務可使用的 HTTP API。

## 功能目標

- 提供 Immersive Translate 自訂 API 相容的 `POST /translate` 端點。
- 接收 `source_lang`、`target_lang`、`text_list`，批次轉送至 Google Translate。
- 回傳 `translations` 陣列，保持與 Immersive Translate 文件一致。
- 限制單次請求 body 最大 1 MiB，避免異常請求占用過多資源。
- 透過環境變數管理 Google API key，不將金鑰提交到版本控制。
- 提供 `GET /healthz` 作為健康檢查。
- 首次啟動若沒有 `.env`，會使用 `.env.example` 範本自動建立。
- 提供 `build.ps1` 編譯目前環境，或依參數編譯 Windows、Linux、macOS 的 x86 與 ARM 執行檔到 `dist` 目錄。
- 提供 GitHub Actions，在推送 `v*` tag 時全平台編譯並上傳 GitHub Release。
- 執行檔可直接安裝、啟動、停止或移除作業系統服務。

## 設定

首次啟動若沒有 `.env`，服務會使用 `.env.example` 範本自動建立。`.env.example` 已提供預設公開 Google Translate API key，也可以自行改用其他 key。

若 `.env` 存在但沒有設定 `GOOGLE_TRANSLATE_API_KEY`，且系統環境變數也沒有設定，服務會使用程式內建的預設公開 API key。

```env
GOOGLE_TRANSLATE_URL=https://translate-pa.googleapis.com/v1/translateHtml
GOOGLE_TRANSLATE_API_KEY=AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520
PORT=8080
```

`.env` 只供本機覆寫設定使用，已由 `.gitignore` 忽略。若改用私人金鑰，請勿提交。

部署環境可直接設定系統環境變數，不一定需要 `.env` 檔案。若要使用自己的 key，請設定 `GOOGLE_TRANSLATE_API_KEY` 覆寫預設值。

## 啟動方式

```powershell
go run .
```

預設監聽連接埠為 `8080`，可透過 `PORT` 覆寫。

執行檔也可明確使用 `run` 前景啟動：

```powershell
.\go-google-translate-proxy.exe run
```

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

macOS 目前在 Go 1.21 僅支援 `amd64` 與 `arm64`。

若要先清空舊的 `dist` 目錄再編譯：

```powershell
.\build.ps1 -Clean -All
```

## 發佈

推送 `v` 開頭的 tag 時，GitHub Actions 會執行測試、全平台編譯，並將 `dist` 目錄內所有檔案上傳到同一個 tag 的 GitHub Release。

```powershell
git tag v1.0.0
git push origin v1.0.0
```

Release workflow 會執行：

```powershell
go test ./...
.\build.ps1 -Clean -All
```

## 測試

```powershell
go test ./...
```

## Immersive Translate 設定

在 Immersive Translate 自訂 API 中設定服務網址：

```text
http://localhost:8080/translate
```

若服務部署在遠端，請改成自己的 HTTPS 網址。

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

## 授權

本專案採用 MIT License。詳見 `LICENSE`。

此授權僅涵蓋本專案程式碼與文件，不代表授權使用 Google Translate API、API key 或 Google 服務本身。
