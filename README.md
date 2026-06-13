# go-google-translate-proxy

將 Google Translate `translateHtml` 端點包裝成 Immersive Translate 自訂翻譯服務可使用的 HTTP API。

## 功能目標

- 提供 Immersive Translate 自訂 API 相容的 `POST /translate` 端點。
- 接收 `source_lang`、`target_lang`、`text_list`，批次轉送至 Google Translate。
- 回傳 `translations` 陣列，保持與 Immersive Translate 文件一致。
- 透過環境變數管理 Google API key，不將金鑰提交到版本控制。
- 提供 `GET /healthz` 作為健康檢查。
- 首次啟動若沒有 `.env`，會使用 `.env.example` 範本自動建立。
- 提供 `build.ps1` 編譯單一執行檔到 `dist` 目錄。

## 設定

首次啟動若沒有 `.env`，服務會使用 `.env.example` 範本自動建立。`.env.example` 已提供預設公開 Google Translate API key，也可以自行改用其他 key。

```env
GOOGLE_TRANSLATE_URL=https://translate-pa.googleapis.com/v1/translateHtml
GOOGLE_TRANSLATE_API_KEY=AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520
PORT=8080
```

`.env` 只供本機覆寫設定使用，已由 `.gitignore` 忽略。若改用私人金鑰，請勿提交。

部署環境可直接設定系統環境變數，不一定需要 `.env` 檔案。

## 啟動方式

```powershell
go run .
```

預設監聽連接埠為 `8080`，可透過 `PORT` 覆寫。

## 編譯

```powershell
.\build.ps1
```

編譯結果會輸出到：

```text
dist\go-google-translate-proxy.exe
```

若要先清空舊的 `dist` 目錄再編譯：

```powershell
.\build.ps1 -Clean
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
