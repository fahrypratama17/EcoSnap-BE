package entity

import "time"

type FavoritePartner struct {
	PartnerID string    `json:"partner_id"`
	CreatedAt time.Time `json:"created_at"`
}
