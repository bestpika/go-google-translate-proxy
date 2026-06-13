# API 規格

## 概述

本服務提供 Immersive Translate 自訂翻譯 API 相容的 HTTP 介面，並將請求轉送到 Google Translate `translateHtml`。

## Base URL

本機預設：

```text
http://localhost:8080
```

## 端點

### `GET /healthz`

健康檢查端點。

#### 成功回應

```json
{
  "status": "ok"
}
```

### `POST /translate`

翻譯文字陣列。

#### Request Headers

```http
Content-Type: application/json
```

#### Request Body

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
| `source_lang` | string | 否 | 來源語言代碼。空值時視為 `auto`。 |
| `target_lang` | string | 是 | 目標語言代碼。 |
| `text_list` | string array | 是 | 待翻譯文字清單，至少一筆。 |

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
| `translations[].detected_source_lang` | string | 偵測或使用的來源語言。 |
| `translations[].text` | string | 翻譯後文字。 |

## 錯誤回應

錯誤回應使用 JSON 格式。

```json
{
  "error": "invalid request body"
}
```

| HTTP 狀態碼 | 情境 |
| --- | --- |
| `400` | JSON 格式錯誤、必要欄位缺漏或 `text_list` 為空。 |
| `405` | 使用不支援的方法。 |
| `500` | 服務設定錯誤，例如未設定 API key。 |
| `502` | Google 上游錯誤、回應格式不符或網路錯誤。 |

## Google 上游映射

Immersive Translate 請求：

```json
{
  "source_lang": "en",
  "target_lang": "zh-TW",
  "text_list": ["Hello world"]
}
```

Google request body：

```json
[
  [["Hello world"], "en", "zh-TW"],
  "wt_lib"
]
```

Google request headers：

```http
Content-Type: application/json+protobuf
X-Goog-API-Key: <GOOGLE_TRANSLATE_API_KEY>
```
