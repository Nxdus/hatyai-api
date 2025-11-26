# Hatyai Flood SOS API Documentation

This document provides instructions for using the Hatyai Flood SOS API to access flood assistance data.

## Endpoints

- `GET /v1`: Returns the raw, unfiltered data feed from the upstream source.
- `GET /v1/health`: Checks the API's connection to the Redis cache. Returns `{"status":"ok"}` on success.
- `GET /v1/province/:name`: Filters data by province name (e.g., `/province/สงขลา`).
- `GET /v1/district/:name`: Filters data by district name (e.g., `/district/หาดใหญ่`).
- `GET /v1/subdistrict/:name`: Filters data by subdistrict name.
- `GET /v1/area_summary`: Provides a summary count of items per province, district, and subdistrict.
- `GET /v1/priority`: Ranks items by urgency for the southern region.
  - **Query Parameters:**
    - `priority_level`: `critical` | `high` | `medium` | `low` | `all`
    - `limit`: (integer) The number of items to return.
- `GET /v1/south`: Returns only items located in the southern region of Thailand.
- `GET /v1/area_summary/south`: Returns an area summary limited to the southern region.

## Notes on Usage

- **Naming:** Only Thai names are supported for filtering (e.g., `/province/สงขลา`). The search is case-insensitive.
- **Spaces in Names:** If a name contains spaces, you must URL-encode it. For example, replace spaces with `%20` or `+`.
- **Data Schema:** The data is passed through from an upstream source. Fields, especially within the `properties` object, may change without notice.

## Sample Requests & Responses

### 1) Health Check
```bash
curl "http://localhost/health"
```
```json
{"status":"ok"}
```

### 2) Filter by Province
```bash
curl "http://localhost/province/สงขลา"
```
```json
{
  "province": "สงขลา",
  "count": 1,
  "items": [
    {
      "_id": "692449f459f42522e305db79",
      "location": { "...": "..." },
      "running_number": "HDY68-1124-0180",
      "updated_at": "2025-11-24T12:05:08.017000Z",
      "created_at": "2025-11-24T12:05:08.017000Z"
    }
  ]
}
```

### 3) Get Priority List
This example fetches the top 2 items with a `critical` priority level.

การให้คะแนน `priority.score` (0-100) เป็น rule-based ตามข้อมูลในฟิลด์ของรายการที่ร้องขอความช่วยเหลือ:
- ระดับอาการป่วย (`sick_level_summary`): 1/2/3/4 ให้ +15/+30/+45/+55 ตามลำดับ
- จำนวนผู้ป่วยหรือผู้ประสบภัย (`patient` หรือจำนวน `victims` ถ้า `patient` เป็น 0): +2 ต่อคน สูงสุดคิด 10 คน (+20)
- อายุ (`ages`): ถ้าอายุน้อยกว่า 6 ปี หรือ 70 ปีขึ้นไป ให้ +8
- โรคที่ระบุ (`disease`): ถ้ามีคำสำคัญที่ระบุภาวะรุนแรง ให้ +8
- ความรุนแรงที่พบบนคำอธิบายอื่นๆ (`other`): ตรวจคำสำคัญ 3 ระดับ (เร่งด่วน/กลาง/ทั่วไป) แล้วให้ +12 / +8 / +5 ตามลำดับ
- เวลาที่อัปเดต (`updated_at`): ถ้าอัปเดตภายใน 24 ชม. ให้ +6; ถ้าเกิน 72 ชม. หัก -5
- จำกัดคะแนนให้อยู่ระหว่าง 0-100 แล้วแปลงเป็น `priority_level`: critical ≥ 75, high ≥ 55, medium ≥ 35, นอกนั้นเป็น low

```bash
curl "http://localhost/v1/priority?limit=2&priority_level=critical"
```

```json
{
  "count": 42,
  "items": [
    {
      "_id": "692449f459f42522e305db79",
      "location": { "...": "..." },
      "running_number": "HDY68-1124-0180",
      "priority": {
        "score": 83,
        "level": "critical",
        "reasons": [
          "<reason 1>",
          "<reason 2>",
          "<reason 3>"
        ]
      }
    }
  ]
}
```

### 4) Get Area Summary
```bash
curl "http://localhost/area_summary"
```
```json
{
  "provinces": { "total": 14, "items": [ { "name": "สงขลา", "count": 10 }, { "name": "หาดใหญ่", "count": 4 } ] },
  "districts": { "total": 32, "items": [ { "name": "หาดใหญ่", "count": 6 }, { "name": "คลองอู่ตะเภา", "count": 1 } ] },
  "subdistricts": { "total": 40, "items": [ { "name": "หาดใหญ่", "count": 3 } ] }
}
```
