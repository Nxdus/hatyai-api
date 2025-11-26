package priority

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Nxdus/hatyai-api/services"
)

type Result struct {
	Score   int      `json:"score"`
	Level   string   `json:"level"`
	Reasons []string `json:"reasons"`
}

func Calculate(prop services.LocationProperty) Result {
	var score float64
	var reasons []string

	otherText := strings.ToLower(prop.Other)

	switch prop.SickLevelSummary {
	case 4:
		score += 55
		reasons = append(reasons, "ระดับความเจ็บป่วย: 4")
	case 3:
		score += 45
		reasons = append(reasons, "ระดับความเจ็บป่วย: 3")
	case 2:
		score += 30
		reasons = append(reasons, "ระดับความเจ็บป่วย: 2")
	case 1:
		score += 15
		reasons = append(reasons, "ระดับความเจ็บป่วย: 1")
	}

	patientCount := prop.Patient
	if patientCount == 0 && len(prop.Victims) > 0 {
		patientCount = len(prop.Victims)
	}
	if patientCount > 0 {
		weight := float64(patientCount)
		if weight > 10 {
			weight = 10
		}
		score += weight * 2
		reasons = append(reasons, "มีผู้ป่วยจำนวน "+strconv.Itoa(patientCount)+" คน")
	}

	age := parseAge(prop.Ages)
	if age > 0 && (age < 6 || age >= 70) {
		score += 8
		reasons = append(reasons, "อายุเสี่ยง: "+strconv.Itoa(age)+" ปี")
	}

	disease := strings.ToLower(prop.Disease)
	severeKeywords := []string{
		"หัวใจ",
		"หัวใจหยุด",
		"เส้นเลือด",
		"หลอดเลือดสมอง",
		"มะเร็ง",
		"ฟอกไต",
		"เครื่องช่วยหายใจ",
	}
	if kw, ok := findKeyword(disease, severeKeywords...); ok {
		score += 8
		reasons = append(reasons, "โรคประจำตัว: "+kw)
	}

	if t, ok := parseTime(prop.UpdatedAt); ok {
		hours := time.Since(t).Hours()
		switch {
		case hours <= 24:
			score += 6
			reasons = append(reasons, "อัพเดตในช่วง 24 ชั่วโมงที่ผ่านมา")
		case hours > 72:
			score -= 5
			reasons = append(reasons, "ไม่มีการอัพเดตเกิน 72 ชั่วโมง")
		}
	}

	if kw, ok := findKeyword(
		otherText,
		"หมดสติ",
		"หัวใจหยุด",
		"วิกฤต",
		"ช่วยด่วน",
		"ฟอกไต",
		"หายใจไม่ออก",
		"เลือดออกมาก",
		"เสียเลือด",
		"หยุดหายใจ",
		"ช็อก",
		"ชัก",
		"ไม่รู้สึกตัว",
		"บาดเจ็บหนัก",
		"กระดูกหัก",
	); ok {
		score += 12
		reasons = append(reasons, "มีคีย์เวิร์ดรุนแรง: "+kw)

	} else if kw, ok := findKeyword(
		otherText,
		"ติดเตียง",
		"พิการ",
		"ใกล้คลอด",
		"เด็กเล็ก",
		"ผู้สูงอายุ",
		"ทารกแรกเกิด",
	); ok {
		score += 8
		reasons = append(reasons, "มีคีย์เวิร์ดเสี่ยง: "+kw)
	} else if kw, ok := findKeyword(
		otherText,
		"ขาดอาหาร",
		"ขาดน้ำ",
		"ขาดยา",
		"ขาดไฟ",
		"ติดต่อไม่ได้",
		"ตัดขาด",
		"ติดอยู่",
		"ไม่มีสัญญาณ",
		"ไฟดับ",
	); ok {
		score += 5
		reasons = append(reasons, "มีคีย์เวิร์ดต้องการความช่วยเหลือ: "+kw)
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	level := "low"
	switch {
	case score >= 75:
		level = "critical"
	case score >= 55:
		level = "high"
	case score >= 35:
		level = "medium"
	}

	return Result{
		Score:   int(math.Round(score)),
		Level:   level,
		Reasons: reasons,
	}
}

func parseAge(val string) int {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}

	builder := strings.Builder{}
	for _, r := range val {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		} else if builder.Len() > 0 {
			break
		}
	}
	if builder.Len() == 0 {
		return 0
	}

	age, err := strconv.Atoi(builder.String())
	if err != nil {
		return 0
	}
	return age
}

func parseTime(val string) (time.Time, bool) {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, false
	}

	t, err := time.Parse(time.RFC3339Nano, val)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func findKeyword(text string, keywords ...string) (string, bool) {
	if text == "" {
		return "", false
	}
	lowerText := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			return kw, true
		}
	}
	return "", false
}
