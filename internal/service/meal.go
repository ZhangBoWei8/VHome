package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"vhome/internal/model"
	"vhome/internal/repository"
)

const mealLocationName = "Asia/Shanghai"

var (
	ErrFoodNameExists = errors.New(
		"service: food name already exists",
	)
	ErrFoodRestoreRequired = errors.New(
		"service: deleted food with the same name must be restored",
	)
	ErrNutritionConfirmationRequired = errors.New(
		"service: nutrition mismatch requires confirmation",
	)
)

type MealService struct {
	repository *repository.Repository
	location   *time.Location
}

func NewMealService(repo *repository.Repository) (*MealService, error) {
	if repo == nil {
		return nil, errors.New("meal service repository is nil")
	}

	location, err := time.LoadLocation(mealLocationName)
	if err != nil {
		return nil, fmt.Errorf(
			"load meal service location %q: %w",
			mealLocationName,
			err,
		)
	}

	return &MealService{
		repository: repo,
		location:   location,
	}, nil
}

type FoodListScope string

const (
	FoodListScopeActive  FoodListScope = "ACTIVE"
	FoodListScopeDeleted FoodListScope = "DELETED"
)

func (s FoodListScope) Valid() bool {
	switch s {
	case FoodListScopeActive, FoodListScopeDeleted:
		return true
	default:
		return false
	}
}

type CreateFoodInput struct {
	Name                     string
	CaloriesPer100G          float64
	CarbohydratePer100G      *float64
	ProteinPer100G           *float64
	FatPer100G               *float64
	IconType                 model.FoodIconType
	IconValue                string
	ConfirmNutritionMismatch bool
}

type UpdateFoodInput struct {
	Name                     string
	CaloriesPer100G          float64
	CarbohydratePer100G      *float64
	ProteinPer100G           *float64
	FatPer100G               *float64
	IconType                 model.FoodIconType
	IconValue                string
	ConfirmNutritionMismatch bool
	Version                  uint64
}

type CreateMealRecordInput struct {
	MealDate    time.Time
	MealType    model.MealType
	FoodID      uint64
	WeightGrams float64
}

type UpdateMealRecordInput struct {
	MealDate    time.Time
	MealType    model.MealType
	FoodID      uint64
	WeightGrams float64
	Version     uint64
}

type FoodView struct {
	model.Food
	NutritionComplete        bool     `json:"nutrition_complete"`
	EstimatedCaloriesPer100G *float64 `json:"estimated_calories_per_100g"`
	NutritionMismatch        bool     `json:"nutrition_mismatch"`
}

type MealRecordView struct {
	model.MealRecord
	Calories            float64  `json:"calories"`
	Carbohydrate        *float64 `json:"carbohydrate"`
	Protein             *float64 `json:"protein"`
	Fat                 *float64 `json:"fat"`
	NutritionIncomplete bool     `json:"nutrition_incomplete"`
}

type DailyMealSummary struct {
	Calories              float64 `json:"calories"`
	Carbohydrate          float64 `json:"carbohydrate"`
	Protein               float64 `json:"protein"`
	Fat                   float64 `json:"fat"`
	NutritionIncomplete   bool    `json:"nutrition_incomplete"`
	IncompleteRecordCount uint64  `json:"incomplete_record_count"`
	RecordCount           uint64  `json:"record_count"`
}

type MealMemberOption struct {
	ID          uint64             `json:"id"`
	DisplayName string             `json:"display_name"`
	AvatarKey   model.MemberAvatar `json:"avatar_key"`
	IsCurrent   bool               `json:"is_current"`
}

type MealDayView struct {
	Date    string           `json:"date"`
	Member  MealMemberOption `json:"member"`
	Summary DailyMealSummary `json:"summary"`
	Records []MealRecordView `json:"records"`
}

type MealCalendarDay struct {
	Date                  string  `json:"date"`
	Calories              float64 `json:"calories"`
	RecordCount           uint64  `json:"record_count"`
	NutritionIncomplete   bool    `json:"nutrition_incomplete"`
	IncompleteRecordCount uint64  `json:"incomplete_record_count"`
}

type MealCalendarView struct {
	MemberID uint64            `json:"member_id"`
	Month    string            `json:"month"`
	Days     []MealCalendarDay `json:"days"`
}

func (m *MealService) ListFoods(ctx context.Context, keywords string) ([]model.Food, error) {
	foods, err := m.repository.ListActiveFoods(ctx, keywords)
	if err != nil {
		return nil, fmt.Errorf("sql exec error: %w", err)
	}
	return foods, err
}

func (m *MealService) CreateFood(ctx context.Context, actor AuthenticatedIdentity, food CreateFoodInput) (FoodView, error) {
	food.Name = strings.TrimSpace(food.Name)
	food.IconValue = strings.TrimSpace(food.IconValue)
	if err := ValidateFoodInput(food); err != nil {
		return FoodView{}, fmt.Errorf("meta data error")
	}

	if nutritionMismatch(food) && !food.ConfirmNutritionMismatch {
		return FoodView{}, ErrNutritionConfirmationRequired
	}

	foodmsg := repository.CreateFoodParams{
		Name:                food.Name,
		CaloriesPer100G:     food.CaloriesPer100G,
		CarbohydratePer100G: food.CarbohydratePer100G,
		FatPer100G:          food.FatPer100G,
		ProteinPer100G:      food.ProteinPer100G,
		IconType:            food.IconType,
		IconValue:           food.IconValue,
		Source:              "",
		CreatedBy:           &actor.MemberID,
	}
	_, err := m.repository.CreateFood(ctx, foodmsg)
}
