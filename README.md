# go-google-translate-proxy

將 Google Translate `translateHtml` 端點包裝成 Immersive Translate 自訂翻譯服務可使用的 HTTP API。

## 功能目標

- 提供 Immersive Translate 自訂 API 相容的 `POST /translate` 端點。
- 接收 `source_lang`、`target_lang`、`text_list`，批次轉送至 Google Translate。
- 回傳 `translations` 陣列，保持與 Immersive Translate 文件一致。
- 透過環境變數管理 Google API key，不將金鑰提交到版本控制。
- 提供 `GET /healthz` 作為健康檢查。

## 設定

複製 `.env.example` 為 `.env`，並填入自己的 Google Translate API key。

```env
GOOGLE_TRANSLATE_API_KEY=your_google_translate_api_key
PORT=8080
```

`.env` 只供本機使用，已由 `.gitignore` 忽略，請勿提交真實金鑰。

部署環境可直接設定系統環境變數，不一定需要 `.env` 檔案。

## 預計啟動方式

```powershell
go run .
```

預設監聽連接埠為 `8080`，可透過 `PORT` 覆寫。

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
