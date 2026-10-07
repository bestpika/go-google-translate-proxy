# API 規格

## 概述

本服務提供 Immersive Translate 自訂翻譯 API 相容的 HTTP 介面，並將請求轉送到 Google Translate `translateHtml`。

## 基底網址

本機預設：

```text
http://localhost:8080
```

## 端點

### `GET /healthz`

存活檢查端點，不呼叫 Google，不保證上游可用。僅接受 GET，其他方法回傳 `405` 與 `Allow: GET`。

#### 成功回應

```json
{
  "status": "ok"
}
```

### `POST /translate`

翻譯文字陣列。

#### 請求標頭

```http
Content-Type: application/json
```

建議傳送 `application/json`；為保留既有用法，服務不強制檢查此標頭，也不新增 `415` 回應。

#### 請求內容

```json
{
  "source_lang": "en",
  "target_lang": "zh-TW",
  "text_list": ["Hello world"]
}
```

#### 欄位

| 欄位 | 型別 | 必填 | 說明 |
| --- | --- | --- | --- |
| `source_lang` | string 或 null | 否 | 去除前後空白後，空值或 `null` 視為 `auto`。 |
| `target_lang` | string | 是 | 去除前後空白後不可為空。 |
| `text_list` | string array | 是 | 至少一筆；每筆不可為空字串，保留文字原始空白。 |

請求內容最大 1 MiB，包含尾端空白；超過時回傳 `413 Payload Too Large`，不呼叫上游。必須是單一 JSON 物件，不接受未知欄位、第二份 JSON 或尾端非空白內容。合法 JSON 後的空白可接受。

#### 成功回應

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

#### 回應欄位

| 欄位 | 型別 | 說明 |
| --- | --- | --- |
| `translations` | object array | 翻譯結果。 |
| `translations[].detected_source_lang` | string | 使用的來源語言；輸入為 `auto` 時回傳 `auto`，並非真正偵測結果。 |
| `translations[].text` | string | 翻譯後文字。 |

翻譯筆數與輸入相同，順序不變。僅接受 POST，其他方法回傳 `405` 與 `Allow: POST`。

## 錯誤回應

錯誤回應使用 JSON 格式。

```json
{
  "error": "invalid request body"
}
```

| HTTP 狀態碼 | 情境 |
| --- | --- |
| `400` | JSON 格式錯誤、未知欄位、尾端額外內容、必要欄位缺漏或空文字。 |
| `413` | 請求 body 超過 1 MiB。 |
| `405` | 使用不支援的方法，包含端點允許方法的 `Allow` 標頭。 |
| `500` | 翻譯器內部金鑰為空；一般未覆寫設定時仍會使用公開預設值。 |
| `502` | Google 上游錯誤、格式或筆數不符、回應超量、網路錯誤、取消或逾時。 |

對外錯誤不包含金鑰、上游回應或譯文。未知路徑保留 Go 標準 `404` 回應，不保證 JSON 格式。

## Google 上游映射

Immersive Translate 請求：

```json
{
  "source_lang": "en",
  "target_lang": "zh-TW",
  "text_list": ["Hello world"]
}
```

Google 請求內容：

```json
[
  [["Hello world"], "en", "zh-TW"],
  "wt_lib"
]
```

Google 請求標頭：

```http
Content-Type: application/json+protobuf
X-Goog-API-Key: <GOOGLE_TRANSLATE_API_KEY>
```

上游回應最大 1 MiB，逾時 30 秒。回應第一個元素必須是與輸入筆數一致的字串陣列；其他中繼資料不作為來源語言偵測結果。不新增重試或快取；自動測試僅呼叫本機模擬上游，不使用真實 Google。
