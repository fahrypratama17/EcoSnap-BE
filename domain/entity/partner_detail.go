package entity

type WasteCategory struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Slug                    string `json:"slug"`
	Code                    string `json:"code"`
	HandlingGuide           string `json:"handling_guide"`
	RequiresSpecialHandling bool   `json:"requires_special_handling"`
}
type PartnerLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type PartnerHour struct {
	DayOfWeek int    `json:"day_of_week"`
	OpensAt   string `json:"opens_at"`
	ClosesAt  string `json:"closes_at"`
	IsClosed  bool   `json:"is_closed"`
}
type PartnerDetail struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Area              string          `json:"area"`
	Address           string          `json:"address"`
	Phone             string          `json:"phone"`
	OpeningHours      string          `json:"opening_hours"`
	Description       string          `json:"description"`
	Location          PartnerLocation `json:"location"`
	AcceptedMaterials []WasteCategory `json:"accepted_materials"`
	Hours             []PartnerHour   `json:"hours"`
}
