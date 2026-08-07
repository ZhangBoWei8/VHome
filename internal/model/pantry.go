package model

import "time"

type StorageType string

const (
	StorageTypeCold    StorageType = "COLD"
	StorageTypeAmbient StorageType = "AMBIENT"
)

type StorageLocation struct {
	ID          uint64      `json:"id"`
	Name        string      `json:"name"`
	IconKey     string      `json:"icon_key"`
	StorageType StorageType `json:"storage_type"`
	IsBuiltin   bool        `json:"is_builtin"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
}

type MaterialTemplate struct {
	ID                   uint64    `json:"id"`
	Name                 string    `json:"name"`
	IconKey              string    `json:"icon_key"`
	DefaultUnit          string    `json:"default_unit"`
	ColdShelfLifeDays    *int      `json:"cold_shelf_life_days"`
	AmbientShelfLifeDays *int      `json:"ambient_shelf_life_days"`
	CaloriesPer100G      *float64  `json:"calories_per_100g"`
	ProteinPer100G       *float64  `json:"protein_per_100g"`
	FatPer100G           *float64  `json:"fat_per_100g"`
	CarbohydratePer100G  *float64  `json:"carbohydrate_per_100g"`
	Source               string    `json:"source"`
	Enabled              bool      `json:"enabled"`
	Version              uint64    `json:"version"`
	CreatedAt, UpdatedAt time.Time `json:"-"`
}

type InventoryItem struct {
	ID                   uint64      `json:"id"`
	TemplateID           *uint64     `json:"template_id"`
	StorageLocationID    uint64      `json:"storage_location_id"`
	Name                 string      `json:"name"`
	IconKey              string      `json:"icon_key"`
	Quantity             float64     `json:"quantity"`
	Unit                 string      `json:"unit"`
	StockedOn            time.Time   `json:"stocked_on"`
	ExpiresOn            time.Time   `json:"expires_on"`
	CaloriesPer100G      *float64    `json:"calories_per_100g"`
	ProteinPer100G       *float64    `json:"protein_per_100g"`
	FatPer100G           *float64    `json:"fat_per_100g"`
	CarbohydratePer100G  *float64    `json:"carbohydrate_per_100g"`
	Description          string      `json:"description"`
	ImagePath            string      `json:"image_path"`
	Status               string      `json:"status"`
	DiscardedAt          *time.Time  `json:"discarded_at,omitempty"`
	DiscardedBy          *uint64     `json:"discarded_by,omitempty"`
	DiscardReason        string      `json:"discard_reason,omitempty"`
	Version              uint64      `json:"version"`
	CreatedBy            uint64      `json:"created_by"`
	CreatedAt, UpdatedAt time.Time   `json:"-"`
	LocationName         string      `json:"location_name"`
	StorageType          StorageType `json:"storage_type"`
}

type ReminderMilestone string

const (
	ReminderHalf          ReminderMilestone = "HALF"
	ReminderThreeQuarters ReminderMilestone = "THREE_QUARTERS"
	ReminderLastDay       ReminderMilestone = "LAST_DAY"
)
