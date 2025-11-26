package routes

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nxdus/hatyai-api/priority"
	"github.com/Nxdus/hatyai-api/services"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(app *fiber.App, sosService services.SOSService, rdb *redis.Client) {
	app.Get("/v1", func(c *fiber.Ctx) error {
		raw, err := sosService.GetRaw()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.Send(raw)
	})

	app.Get("/v1/health", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			return c.Status(503).JSON(fiber.Map{"redis": "down"})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/v1/province/:name", func(c *fiber.Ctx) error {
		name := decodeParam(c.Params("name"))
		if strings.TrimSpace(name) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "province is required"})
		}

		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := filterItems(data.Data.Data, name, func(item services.DataItem) string {
			return item.Location.Properties.Province
		})

		return c.JSON(fiber.Map{
			"province": name,
			"count":    len(items),
			"items":    items,
		})
	})

	app.Get("/v1/district/:name", func(c *fiber.Ctx) error {
		name := decodeParam(c.Params("name"))
		if strings.TrimSpace(name) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "district is required"})
		}

		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := filterItems(data.Data.Data, name, func(item services.DataItem) string {
			return item.Location.Properties.District
		})

		return c.JSON(fiber.Map{
			"district": name,
			"count":    len(items),
			"items":    items,
		})
	})

	app.Get("/v1/subdistrict/:name", func(c *fiber.Ctx) error {
		name := decodeParam(c.Params("name"))
		if strings.TrimSpace(name) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subdistrict is required"})
		}

		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := filterItems(data.Data.Data, name, func(item services.DataItem) string {
			return item.Location.Properties.SubDistrict
		})

		return c.JSON(fiber.Map{
			"subdistrict": name,
			"count":       len(items),
			"items":       items,
		})
	})

	app.Get("/v1/area_summary", func(c *fiber.Ctx) error {
		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := data.Data.Data
		provinceCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.Province })
		districtCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.District })
		subdistrictCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.SubDistrict })

		return c.JSON(fiber.Map{
			"provinces":    fiber.Map{"total": len(provinceCounts), "items": provinceCounts},
			"districts":    fiber.Map{"total": len(districtCounts), "items": districtCounts},
			"subdistricts": fiber.Map{"total": len(subdistrictCounts), "items": subdistrictCounts},
		})
	})

	app.Get("/v1/priority", func(c *fiber.Ctx) error {
		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := make([]prioritizedDataItem, 0, len(filterItemsByLatLon(filterItemsByProvince(data.Data.Data, isSouthernProvince), InSouthernThailand)))

		for _, item := range filterItemsByLatLon(filterItemsByProvince(data.Data.Data, isSouthernProvince), InSouthernThailand) {
			items = append(items, prioritizedDataItem{
				DataItem: item,
				Priority: priority.Calculate(item.Location.Properties),
			})
		}

		levelFilter := strings.ToLower(strings.TrimSpace(c.Query("priority_level")))
		if levelFilter != "" && levelFilter != "all" {
			filtered := make([]prioritizedDataItem, 0, len(items))
			for _, it := range items {
				if strings.ToLower(it.Priority.Level) == levelFilter {
					filtered = append(filtered, it)
				}
			}
			items = filtered
		}

		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Priority.Score == items[j].Priority.Score {
				return mostRecentUpdate(items[i]).After(mostRecentUpdate(items[j]))
			}
			return items[i].Priority.Score > items[j].Priority.Score
		})

		limit := len(items)
		if q := strings.TrimSpace(c.Query("limit")); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 && n < limit {
				limit = n
			}
		}

		return c.JSON(fiber.Map{
			"count": len(items),
			"items": items[:limit],
		})
	})

	app.Get("/v1/south", func(c *fiber.Ctx) error {
		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := filterItemsByLatLon(filterItemsByProvince(data.Data.Data, isSouthernProvince), InSouthernThailand)

		return c.JSON(fiber.Map{
			"count": len(items),
			"items": items,
		})
	})

	app.Get("/v1/area_summary/south", func(c *fiber.Ctx) error {
		data, err := sosService.GetSOS()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		items := filterItemsByProvince(data.Data.Data, isSouthernProvince)
		provinceCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.Province })
		districtCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.District })
		subdistrictCounts := buildCounts(items, func(item services.DataItem) string { return item.Location.Properties.SubDistrict })

		return c.JSON(fiber.Map{
			"region":       "south",
			"provinces":    fiber.Map{"total": len(provinceCounts), "items": provinceCounts},
			"districts":    fiber.Map{"total": len(districtCounts), "items": districtCounts},
			"subdistricts": fiber.Map{"total": len(subdistrictCounts), "items": subdistrictCounts},
		})
	})
}

type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var southernProvinces = map[string]struct{}{
	"phuket":                               {},
	"krabi":                                {},
	"phang nga":                            {},
	"ranong":                               {},
	"chumphon":                             {},
	"surat thani":                          {},
	"nakhon si thammarat":                  {},
	"phatthalung":                          {},
	"trang":                                {},
	"satun":                                {},
	"songkhla":                             {},
	"pattani":                              {},
	"yala":                                 {},
	"narathiwat":                           {},
	"\u0e20\u0e39\u0e40\u0e01\u0e47\u0e15": {}, // Thai: \u0e20\u0e39\u0e40\u0e01\u0e47\u0e15 (Phuket)
	"\u0e01\u0e23\u0e30\u0e1a\u0e35\u0e48": {}, // Thai: \u0e01\u0e23\u0e30\u0e1a\u0e35\u0e48 (Krabi)
	"\u0e1e\u0e31\u0e07\u0e07\u0e32":       {}, // Thai: \u0e1e\u0e31\u0e07\u0e07\u0e32 (Phang Nga)
	"\u0e23\u0e30\u0e19\u0e2d\u0e07":       {}, // Thai: \u0e23\u0e30\u0e19\u0e2d\u0e07 (Ranong)
	"\u0e0a\u0e38\u0e21\u0e1e\u0e23":       {}, // Thai: \u0e0a\u0e38\u0e21\u0e1e\u0e23 (Chumphon)
	"\u0e2a\u0e38\u0e23\u0e32\u0e29\u0e11\u0e23\u0e4c\u0e18\u0e32\u0e19\u0e35":       {}, // Thai: \u0e2a\u0e38\u0e23\u0e32\u0e29\u0e11\u0e23\u0e4c\u0e18\u0e32\u0e19\u0e35 (Surat Thani)
	"\u0e19\u0e04\u0e23\u0e28\u0e23\u0e35\u0e18\u0e23\u0e23\u0e21\u0e23\u0e32\u0e0a": {}, // Thai: \u0e19\u0e04\u0e23\u0e28\u0e23\u0e35\u0e18\u0e23\u0e23\u0e21\u0e23\u0e32\u0e0a (Nakhon Si Thammarat)
	"\u0e1e\u0e31\u0e17\u0e25\u0e38\u0e07":                                           {}, // Thai: \u0e1e\u0e31\u0e17\u0e25\u0e38\u0e07 (Phatthalung)
	"\u0e15\u0e23\u0e31\u0e07":                                                       {}, // Thai: \u0e15\u0e23\u0e31\u0e07 (Trang)
	"\u0e2a\u0e15\u0e39\u0e25":                                                       {}, // Thai: \u0e2a\u0e15\u0e39\u0e25 (Satun)
	"\u0e2a\u0e07\u0e02\u0e25\u0e32":                                                 {}, // Thai: \u0e2a\u0e07\u0e02\u0e25\u0e32 (Songkhla)
	"\u0e1b\u0e31\u0e15\u0e15\u0e32\u0e19\u0e35":                                     {}, // Thai: \u0e1b\u0e31\u0e15\u0e15\u0e32\u0e19\u0e35 (Pattani)
	"\u0e22\u0e30\u0e25\u0e32":                                                       {}, // Thai: \u0e22\u0e30\u0e25\u0e32 (Yala)
	"\u0e19\u0e23\u0e32\u0e18\u0e34\u0e27\u0e32\u0e2a":                               {}, // Thai: \u0e19\u0e23\u0e32\u0e18\u0e34\u0e27\u0e32\u0e2a (Narathiwat)
}

var SouthernPolygon = [][2]float64{
	{98.20, 8.30},  // Phuket NW
	{98.30, 7.70},  // Phuket South
	{98.45, 7.20},  // Krabi
	{98.80, 6.80},  // Trang
	{99.10, 6.50},  // Satun
	{100.00, 5.60}, // Yala South
	{101.30, 5.75}, // Narathiwat South
	{102.10, 6.50}, // Gulf East
	{102.10, 7.80}, // Narathiwat East
	{101.90, 8.80}, // Nakhon Si Thammarat
	{101.50, 9.60},
	{101.00, 10.50}, // Surat Thani
	{100.50, 11.10}, // Chumphon
	{99.50, 11.10},
	{98.80, 10.50},
	{98.30, 9.50}, // Ranong
	{98.20, 8.80}, // Back to Andaman
}

func PointInPolygon(lat, lon float64, polygon [][2]float64) bool {
	inside := false
	n := len(polygon)

	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]

		intersect := ((yi > lat) != (yj > lat)) &&
			(lon < (xj-xi)*(lat-yi)/(yj-yi)+xi)

		if intersect {
			inside = !inside
		}
	}

	return inside
}

func InSouthernThailand(lat, lon float64) bool {
	if lat < 5.6 || lat > 11.1 {
		return false
	}
	if lon < 98.3 || lon > 102.1 {
		return false
	}

	return PointInPolygon(lat, lon, SouthernPolygon)
}

func filterItems(items []services.DataItem, target string, get func(services.DataItem) string) []services.DataItem {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return nil
	}

	filtered := make([]services.DataItem, 0)
	for _, item := range items {
		if strings.ToLower(get(item)) == target {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterItemsByProvince(items []services.DataItem, allow func(string) bool) []services.DataItem {
	filtered := make([]services.DataItem, 0)
	for _, item := range items {
		prov := strings.ToLower(strings.TrimSpace(item.Location.Properties.Province))
		if allow(prov) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterItemsByLatLon(items []services.DataItem, allow func(float64, float64) bool) []services.DataItem {
	filtered := make([]services.DataItem, 0)
	for _, item := range items {
		lat, long := item.Location.Geometry.Coordinates[1], item.Location.Geometry.Coordinates[0]
		if allow(lat, long) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func buildCounts(items []services.DataItem, get func(services.DataItem) string) []nameCount {
	temp := make(map[string]*nameCount)
	for _, item := range items {
		name := strings.TrimSpace(get(item))
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if existing, ok := temp[key]; ok {
			existing.Count++
		} else {
			temp[key] = &nameCount{Name: name, Count: 1}
		}
	}

	result := make([]nameCount, 0, len(temp))
	for _, v := range temp {
		result = append(result, *v)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func isSouthernProvince(province string) bool {
	_, ok := southernProvinces[strings.ToLower(strings.TrimSpace(province))]
	return ok
}

func decodeParam(val string) string {
	if val == "" {
		return val
	}
	if decoded, err := url.PathUnescape(val); err == nil {
		return decoded
	}
	return val
}

type prioritizedDataItem struct {
	services.DataItem
	Priority priority.Result `json:"priority"`
}

func mostRecentUpdate(item prioritizedDataItem) time.Time {
	if t, ok := parseUpdatedAt(item.UpdatedAt); ok {
		return t
	}
	if t, ok := parseUpdatedAt(item.Location.Properties.UpdatedAt); ok {
		return t
	}
	return time.Time{}
}

func parseUpdatedAt(val string) (time.Time, bool) {
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
