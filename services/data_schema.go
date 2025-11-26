package services

type APIResponse struct {
	FetchedAt string     `json:"fetched_at"`
	Data      NestedData `json:"data"`
}

type NestedData struct {
	Data []DataItem `json:"data"`
}

type DataItem struct {
	ID            string   `json:"_id"`
	Location      Location `json:"location"`
	RunningNumber string   `json:"running_number"`
	UpdatedAt     string   `json:"updated_at"`
	CreatedAt     string   `json:"created_at"`
}

type Location struct {
	Type       string           `json:"type"`
	Properties LocationProperty `json:"properties"`
	Geometry   Geometry         `json:"geometry"`
}

type LocationProperty struct {
	Other            string        `json:"other"`
	Victims          []interface{} `json:"victims"`
	Patient          int           `json:"patient"`
	Province         string        `json:"province"`
	District         string        `json:"district"`
	SubDistrict      string        `json:"subdistrict"`
	SickLevelSummary int           `json:"sick_level_summary"`
	RunningNumber    string        `json:"running_number"`
	StatusText       string        `json:"status_text"`
	TypeName         string        `json:"type_name"`
	Ages             string        `json:"ages"`
	Disease          string        `json:"disease"`
	UpdatedAt        string        `json:"updated_at"`
}

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}
