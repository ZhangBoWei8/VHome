package model

import "time"

type FoodIconType string

const (
	FoodIconTypeBuiltin FoodIconType = "BUILTIN"
	FoodIconTypeUpload  FoodIconType = "UPLOAD"
)

func (t FoodIconType) Valid() bool {
	switch t {
	case FoodIconTypeBuiltin, FoodIconTypeUpload:
		return true
	default:
		return false
	}
}

type FoodSource string

const (
	FoodSourceBuiltin FoodSource = "BUILTIN"
	FoodSourceUser    FoodSource = "USER"
)

func (s FoodSource) Valid() bool {
	switch s {
	case FoodSourceBuiltin, FoodSourceUser:
		return true
	default:
		return false
	}
}

type MealType string

const (
	MealTypeBreakfast MealType = "BREAKFAST"
	MealTypeLunch     MealType = "LUNCH"
	MealTypeDinner    MealType = "DINNER"
	MealTypeSnack     MealType = "SNACK"
)

func (t MealType) Valid() bool {
	switch t {
	case MealTypeBreakfast, MealTypeLunch, MealTypeDinner, MealTypeSnack:
		return true
	default:
		return false
	}
}

type Food struct {
	ID                  uint64       `json:"id"`
	Name                string       `json:"name"`
	CaloriesPer100G     float64      `json:"calories_per_100g"`
	CarbohydratePer100G *float64     `json:"carbohydrate_per_100g"`
	ProteinPer100G      *float64     `json:"protein_per_100g"`
	FatPer100G          *float64     `json:"fat_per_100g"`
	IconType            FoodIconType `json:"icon_type"`
	IconValue           string       `json:"icon_value"`
	Source              FoodSource   `json:"source"`
	CreatedBy           *uint64      `json:"created_by"`
	DeletedBy           *uint64      `json:"deleted_by"`
	Version             uint64       `json:"version"`
	DeletedAt           *time.Time   `json:"deleted_at"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type MealRecord struct {
	ID                          uint64       `json:"id"`
	MemberID                    uint64       `json:"member_id"`
	MealDate                    time.Time    `json:"meal_date"`
	MealType                    MealType     `json:"meal_type"`
	FoodID                      *uint64      `json:"food_id"`
	FoodNameSnapshot            string       `json:"food_name_snapshot"`
	IconTypeSnapshot            FoodIconType `json:"icon_type_snapshot"`
	IconValueSnapshot           string       `json:"icon_value_snapshot"`
	WeightGrams                 float64      `json:"weight_grams"`
	CaloriesPer100GSnapshot     float64      `json:"calories_per_100g_snapshot"`
	CarbohydratePer100GSnapshot *float64     `json:"carbohydrate_per_100g_snapshot"`
	ProteinPer100GSnapshot      *float64     `json:"protein_per_100g_snapshot"`
	FatPer100GSnapshot          *float64     `json:"fat_per_100g_snapshot"`
	CreatedBy                   uint64       `json:"created_by"`
	DeletedBy                   *uint64      `json:"deleted_by"`
	Version                     uint64       `json:"version"`
	DeletedAt                   *time.Time   `json:"deleted_at"`
	CreatedAt                   time.Time    `json:"created_at"`
	UpdatedAt                   time.Time    `json:"updated_at"`
}
