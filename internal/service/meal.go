package service

import (
	"context"
	"errors"
	"fmt"
	"math"
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

func canManageFood(actor AuthenticatedIdentity, food model.Food) bool {
	switch actor.Role {
	case model.MemberRoleOwner, model.MemberRoleAdmin:
		return true
	case model.MemberRoleMember:
		return food.Source == model.FoodSourceUser && food.CreatedBy != nil && *food.CreatedBy == actor.MemberID
	default:
		return false
	}
}

func (m *MealService) ListFoods(ctx context.Context, keywords string) ([]FoodView, error) {
	keywords = strings.TrimSpace(keywords)
	foods, err := m.repository.ListActiveFoods(ctx, keywords)
	if err != nil {
		return nil, fmt.Errorf("sql exec error: %w", err)
	}
	results := make([]FoodView, 0, len(foods))
	for _, food := range foods {
		results = append(results, buildFoodView(food))
	}
	return results, nil
}

// ActiveFood returns one selectable food. Besides supporting the edit form,
// keeping this lookup in the service layer lets HTTP handlers inspect the old
// uploaded icon without reaching into the repository directly.
func (m *MealService) ActiveFood(ctx context.Context, actor AuthenticatedIdentity, foodID uint64) (FoodView, error) {
	if actor.MemberID == 0 {
		return FoodView{}, ErrUnauthenticated
	}
	if foodID == 0 {
		return FoodView{}, ErrInvalidInput
	}
	food, err := m.repository.GetActiveFoodByID(ctx, foodID)
	if errors.Is(err, repository.ErrNotFound) {
		return FoodView{}, ErrNotFound
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("get active food: %w", err)
	}
	return buildFoodView(food), nil
}

func (m *MealService) CreateFood(ctx context.Context, actor AuthenticatedIdentity, food CreateFoodInput) (FoodView, error) {
	if actor.MemberID == 0 {
		return FoodView{}, ErrUnauthenticated
	}
	food.Name = strings.TrimSpace(food.Name)
	food.IconValue = strings.TrimSpace(food.IconValue)
	if err := validateFoodInput(food); err != nil {
		return FoodView{}, err
	}

	if nutritionMismatch(food) && !food.ConfirmNutritionMismatch {
		return FoodView{}, ErrNutritionConfirmationRequired
	}

	if err := m.checkFoodNameAvailability(ctx, food.Name); err != nil {
		return FoodView{}, err
	}
	creatorID := actor.MemberID

	foodmsg := repository.CreateFoodParams{
		Name:                food.Name,
		CaloriesPer100G:     food.CaloriesPer100G,
		CarbohydratePer100G: food.CarbohydratePer100G,
		FatPer100G:          food.FatPer100G,
		ProteinPer100G:      food.ProteinPer100G,
		IconType:            food.IconType,
		IconValue:           food.IconValue,
		Source:              model.FoodSourceUser,
		CreatedBy:           &creatorID,
	}
	created, err := m.repository.CreateFood(ctx, foodmsg)
	if errors.Is(err, repository.ErrConflict) {
		nameErr := m.checkFoodNameAvailability(ctx, food.Name)
		if nameErr != nil {
			return FoodView{}, nameErr
		}
		return FoodView{}, ErrConflict
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("create food: %w", err)
	}
	return buildFoodView(created), nil
}

func (m *MealService) UpdateFood(ctx context.Context, actor AuthenticatedIdentity, foodID uint64, foodmsg UpdateFoodInput) (FoodView, error) {
	if actor.MemberID == 0 {
		return FoodView{}, ErrUnauthenticated
	}
	if foodID == 0 {
		return FoodView{}, ErrInvalidInput
	}
	current, err := m.repository.GetActiveFoodByID(ctx, foodID)
	if errors.Is(err, repository.ErrNotFound) {
		return FoodView{}, ErrNotFound
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("get active for update: %w", err)
	}

	if !canManageFood(actor, current) {
		return FoodView{}, ErrForbidden
	}

	foodmsg.Name = strings.TrimSpace(foodmsg.Name)
	foodmsg.IconValue = strings.TrimSpace(foodmsg.IconValue)
	// An uploaded icon can be kept without trusting a client-supplied path.
	if foodmsg.IconType == model.FoodIconTypeUpload && foodmsg.IconValue == "" && current.IconType == model.FoodIconTypeUpload {
		foodmsg.IconValue = current.IconValue
	}
	if err := validateUpdateFoodInput(foodmsg); err != nil {
		return FoodView{}, err
	}
	_, _, mismatch := analyzeNutrition(foodmsg.CaloriesPer100G, foodmsg.CarbohydratePer100G, foodmsg.ProteinPer100G, foodmsg.FatPer100G)
	if mismatch && !foodmsg.ConfirmNutritionMismatch {
		return FoodView{}, ErrNutritionConfirmationRequired
	}
	if err := m.checkFoodNameForUpdate(ctx, foodID, foodmsg.Name); err != nil {
		return FoodView{}, err
	}
	updated, err := m.repository.UpdateFood(ctx, repository.UpdateFoodParams{
		ID:                  foodID,
		Name:                foodmsg.Name,
		CaloriesPer100G:     foodmsg.CaloriesPer100G,
		CarbohydratePer100G: foodmsg.CarbohydratePer100G,
		ProteinPer100G:      foodmsg.ProteinPer100G,
		FatPer100G:          foodmsg.FatPer100G,
		IconType:            foodmsg.IconType,
		IconValue:           foodmsg.IconValue,
		Version:             foodmsg.Version,
	})
	if errors.Is(err, repository.ErrConflict) {
		existing, lookupErr := m.repository.GetFoodByName(ctx, foodmsg.Name)
		if lookupErr == nil && existing.ID != foodID {
			if existing.DeletedAt != nil {
				return FoodView{}, ErrFoodRestoreRequired
			}
			return FoodView{}, ErrFoodNameExists
		}
		return FoodView{}, ErrConflict
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("update food: %w", err)
	}
	return buildFoodView(updated), nil
}

func (m *MealService) DeleteFood(ctx context.Context, actor AuthenticatedIdentity, foodID uint64, version uint64) (FoodView, error) {
	if actor.MemberID == 0 {
		return FoodView{}, ErrUnauthenticated
	}

	if foodID == 0 || version == 0 {
		return FoodView{}, ErrInvalidInput
	}
	current, err := m.repository.GetActiveFoodByID(ctx, foodID)
	if errors.Is(err, repository.ErrNotFound) {
		return FoodView{}, ErrNotFound
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("get active for delete: %w", err)
	}
	if !canManageFood(actor, current) {
		return FoodView{}, ErrForbidden
	}
	deleted, err := m.repository.SoftDeleteFood(ctx, repository.SoftDeleteFoodParams{
		ID:        foodID,
		Version:   version,
		DeletedBy: actor.MemberID,
	})
	if errors.Is(err, repository.ErrConflict) {
		return FoodView{}, ErrConflict
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("delete food: %w", err)
	}
	return buildFoodView(deleted), nil
}

func (m *MealService) RestoreFood(ctx context.Context, actor AuthenticatedIdentity, foodID uint64, version uint64) (FoodView, error) {
	if actor.MemberID == 0 {
		return FoodView{}, ErrUnauthenticated
	}
	if foodID == 0 || version == 0 {
		return FoodView{}, ErrInvalidInput
	}
	current, err := m.repository.GetFoodByID(ctx, foodID)
	if errors.Is(err, repository.ErrNotFound) {
		return FoodView{}, ErrNotFound
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("get food for restore: %w", err)
	}
	if current.DeletedAt == nil {
		return FoodView{}, ErrConflict
	}
	if !canManageFood(actor, current) {
		return FoodView{}, ErrForbidden
	}

	restored, err := m.repository.RestoreFood(ctx, repository.RestoreFoodParams{
		ID:      foodID,
		Version: version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return FoodView{}, ErrConflict
	}
	if err != nil {
		return FoodView{}, fmt.Errorf("restore food: %w", err)
	}
	return buildFoodView(restored), nil
}

func (m *MealService) ListDeletedFoods(ctx context.Context, actor AuthenticatedIdentity, keyword string) ([]FoodView, error) {
	if actor.MemberID == 0 {
		return nil, ErrUnauthenticated
	}
	keyword = strings.TrimSpace(keyword)
	foods, err := m.repository.ListDeletedFoods(ctx, keyword)
	if err != nil {
		return nil, fmt.Errorf("list deleted foods: %w", err)
	}
	result := make([]FoodView, 0, len(foods))
	for _, food := range foods {
		if !canManageFood(actor, food) {
			continue
		}
		result = append(result, buildFoodView(food))
	}
	return result, nil
}

// 校验三大营养物质是否和热量所匹配
func nutritionMismatch(food CreateFoodInput) bool {
	_, _, mismatch := analyzeNutrition(food.CaloriesPer100G, food.CarbohydratePer100G, food.ProteinPer100G, food.FatPer100G)
	return mismatch
}

func analyzeNutrition(calories float64, carbohydrate *float64, protein *float64, fat *float64) (complete bool, estimatedCalories *float64, mismatch bool) {
	if carbohydrate == nil || protein == nil || fat == nil {
		return false, nil, false
	}
	estimated := *carbohydrate*4 + *protein*4 + *fat*9
	allowDifference := math.Max(20, estimated*0.20)
	mismatch = math.Abs(calories-estimated) > allowDifference
	rounded := roundNutrition(estimated)
	return true, &rounded, mismatch
}

func roundNutrition(value float64) float64 {
	return math.Round(value*100) / 100
}

func buildFoodView(food model.Food) FoodView {
	complete, estimated, mismatch := analyzeNutrition(food.CaloriesPer100G, food.CarbohydratePer100G, food.ProteinPer100G, food.FatPer100G)
	return FoodView{
		Food:                     food,
		NutritionComplete:        complete,
		EstimatedCaloriesPer100G: estimated,
		NutritionMismatch:        mismatch,
	}
}

func (s *MealService) checkFoodNameAvailability(ctx context.Context, name string) error {
	existing, err := s.repository.GetFoodByName(ctx, name)
	switch {
	case err == nil && existing.DeletedAt == nil:
		return ErrFoodNameExists
	case err == nil && existing.DeletedAt != nil:
		return ErrFoodRestoreRequired
	case errors.Is(err, repository.ErrNotFound):
		return nil
	case err != nil:
		return fmt.Errorf("check food name availability: %w", err)
	}
	return nil
}

func (s *MealService) checkFoodNameForUpdate(ctx context.Context, foodID uint64, name string) error {
	existing, err := s.repository.GetFoodByName(ctx, name)

	switch {
	case err == nil && existing.ID == foodID:
		return nil

	case err == nil && existing.DeletedAt == nil:
		return ErrFoodNameExists

	case err == nil && existing.DeletedAt != nil:
		return ErrFoodRestoreRequired

	case errors.Is(err, repository.ErrNotFound):
		return nil

	case err != nil:
		return fmt.Errorf("check food name for update: %w", err)

	default:
		return nil
	}
}

// normalizeOwnMealDate converts a date-only value to the household timezone
// and rejects future dates. The clock comes from the server, so changing a
// browser's local time cannot bypass this rule.
func (s *MealService) normalizeOwnMealDate(value time.Time) (time.Time, error) {
	if value.IsZero() {
		return time.Time{}, ErrInvalidInput
	}
	normalized := time.Date(
		value.Year(), value.Month(), value.Day(),
		0, 0, 0, 0, s.location,
	)
	if normalized.After(s.today()) {
		return time.Time{}, fmt.Errorf("%w: meal date cannot be in the future", ErrInvalidInput)
	}
	return normalized, nil
}

func (s *MealService) today() time.Time {
	now := time.Now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}

func validateMealValues(mealType model.MealType, foodID uint64, weightGrams float64, version *uint64) error {
	if !mealType.Valid() || foodID == 0 ||
		math.IsNaN(weightGrams) || math.IsInf(weightGrams, 0) ||
		weightGrams <= 0 || weightGrams > 100000 {
		return ErrInvalidInput
	}
	if version != nil && *version == 0 {
		return ErrInvalidInput
	}
	return nil
}

// CreateMyMealRecord derives member ownership from the session and copies the
// selected food into a snapshot. Editing the food later cannot rewrite this
// historical intake record.
func (s *MealService) CreateMyMealRecord(ctx context.Context, actor AuthenticatedIdentity, input CreateMealRecordInput) (MealRecordView, error) {
	if actor.MemberID == 0 {
		return MealRecordView{}, ErrUnauthenticated
	}
	mealDate, err := s.normalizeOwnMealDate(input.MealDate)
	if err != nil {
		return MealRecordView{}, err
	}
	if err := validateMealValues(input.MealType, input.FoodID, input.WeightGrams, nil); err != nil {
		return MealRecordView{}, err
	}

	food, err := s.repository.GetActiveFoodByID(ctx, input.FoodID)
	if errors.Is(err, repository.ErrNotFound) {
		return MealRecordView{}, ErrNotFound
	}
	if err != nil {
		return MealRecordView{}, fmt.Errorf("get food for meal record: %w", err)
	}
	foodID := food.ID
	record, err := s.repository.CreateMealRecord(ctx, repository.CreateMealRecordParams{
		MemberID:                    actor.MemberID,
		MealDate:                    mealDate,
		MealType:                    input.MealType,
		FoodID:                      &foodID,
		FoodNameSnapshot:            food.Name,
		IconTypeSnapshot:            food.IconType,
		IconValueSnapshot:           food.IconValue,
		WeightGrams:                 input.WeightGrams,
		CaloriesPer100GSnapshot:     food.CaloriesPer100G,
		CarbohydratePer100GSnapshot: food.CarbohydratePer100G,
		ProteinPer100GSnapshot:      food.ProteinPer100G,
		FatPer100GSnapshot:          food.FatPer100G,
		CreatedBy:                   actor.MemberID,
	})
	if err != nil {
		return MealRecordView{}, fmt.Errorf("create meal record: %w", err)
	}
	return buildMealRecordView(record), nil
}

// UpdateMyMealRecord preserves the original nutrition snapshot when the food
// is unchanged. Selecting a different food intentionally creates a new
// snapshot from that food's current profile.
func (s *MealService) UpdateMyMealRecord(ctx context.Context, actor AuthenticatedIdentity, recordID uint64, input UpdateMealRecordInput) (MealRecordView, error) {
	if actor.MemberID == 0 {
		return MealRecordView{}, ErrUnauthenticated
	}
	if recordID == 0 {
		return MealRecordView{}, ErrInvalidInput
	}
	mealDate, err := s.normalizeOwnMealDate(input.MealDate)
	if err != nil {
		return MealRecordView{}, err
	}
	if err := validateMealValues(input.MealType, input.FoodID, input.WeightGrams, &input.Version); err != nil {
		return MealRecordView{}, err
	}

	current, err := s.repository.GetActiveMealRecordByID(ctx, recordID, actor.MemberID)
	if errors.Is(err, repository.ErrNotFound) {
		return MealRecordView{}, ErrNotFound
	}
	if err != nil {
		return MealRecordView{}, fmt.Errorf("get meal record for update: %w", err)
	}
	params := repository.UpdateMealRecordParams{
		ID:                          recordID,
		MemberID:                    actor.MemberID,
		MealDate:                    mealDate,
		MealType:                    input.MealType,
		FoodID:                      current.FoodID,
		FoodNameSnapshot:            current.FoodNameSnapshot,
		IconTypeSnapshot:            current.IconTypeSnapshot,
		IconValueSnapshot:           current.IconValueSnapshot,
		WeightGrams:                 input.WeightGrams,
		CaloriesPer100GSnapshot:     current.CaloriesPer100GSnapshot,
		CarbohydratePer100GSnapshot: current.CarbohydratePer100GSnapshot,
		ProteinPer100GSnapshot:      current.ProteinPer100GSnapshot,
		FatPer100GSnapshot:          current.FatPer100GSnapshot,
		Version:                     input.Version,
	}
	if current.FoodID == nil || *current.FoodID != input.FoodID {
		food, err := s.repository.GetActiveFoodByID(ctx, input.FoodID)
		if errors.Is(err, repository.ErrNotFound) {
			return MealRecordView{}, ErrNotFound
		}
		if err != nil {
			return MealRecordView{}, fmt.Errorf("get replacement food: %w", err)
		}
		foodID := food.ID
		params.FoodID = &foodID
		params.FoodNameSnapshot = food.Name
		params.IconTypeSnapshot = food.IconType
		params.IconValueSnapshot = food.IconValue
		params.CaloriesPer100GSnapshot = food.CaloriesPer100G
		params.CarbohydratePer100GSnapshot = food.CarbohydratePer100G
		params.ProteinPer100GSnapshot = food.ProteinPer100G
		params.FatPer100GSnapshot = food.FatPer100G
	}

	updated, err := s.repository.UpdateMealRecord(ctx, params)
	if errors.Is(err, repository.ErrConflict) {
		return MealRecordView{}, ErrConflict
	}
	if err != nil {
		return MealRecordView{}, fmt.Errorf("update meal record: %w", err)
	}
	return buildMealRecordView(updated), nil
}

// DeleteMyMealRecord uses the current member ID in the update predicate, so a
// record ID from another member cannot be deleted even if guessed correctly.
func (s *MealService) DeleteMyMealRecord(ctx context.Context, actor AuthenticatedIdentity, recordID, version uint64) error {
	if actor.MemberID == 0 {
		return ErrUnauthenticated
	}
	if recordID == 0 || version == 0 {
		return ErrInvalidInput
	}
	if _, err := s.repository.GetActiveMealRecordByID(ctx, recordID, actor.MemberID); errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("get meal record for deletion: %w", err)
	}

	err := s.repository.SoftDeleteMealRecord(ctx, repository.SoftDeleteMealRecordParams{
		ID:        recordID,
		MemberID:  actor.MemberID,
		DeletedBy: actor.MemberID,
		Version:   version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("delete meal record: %w", err)
	}
	return nil
}

func buildMealRecordView(record model.MealRecord) MealRecordView {
	factor := record.WeightGrams / 100
	view := MealRecordView{
		MealRecord: record,
		Calories:   roundNutrition(factor * record.CaloriesPer100GSnapshot),
		NutritionIncomplete: record.CarbohydratePer100GSnapshot == nil ||
			record.ProteinPer100GSnapshot == nil || record.FatPer100GSnapshot == nil,
	}
	if record.CarbohydratePer100GSnapshot != nil {
		value := roundNutrition(factor * *record.CarbohydratePer100GSnapshot)
		view.Carbohydrate = &value
	}
	if record.ProteinPer100GSnapshot != nil {
		value := roundNutrition(factor * *record.ProteinPer100GSnapshot)
		view.Protein = &value
	}
	if record.FatPer100GSnapshot != nil {
		value := roundNutrition(factor * *record.FatPer100GSnapshot)
		view.Fat = &value
	}
	return view
}

func buildMealDayView(member MealMemberOption, date time.Time, records []model.MealRecord) MealDayView {
	views := make([]MealRecordView, 0, len(records))
	summary := DailyMealSummary{RecordCount: uint64(len(records))}
	for _, record := range records {
		view := buildMealRecordView(record)
		views = append(views, view)
		summary.Calories += view.Calories
		if view.Carbohydrate != nil {
			summary.Carbohydrate += *view.Carbohydrate
		}
		if view.Protein != nil {
			summary.Protein += *view.Protein
		}
		if view.Fat != nil {
			summary.Fat += *view.Fat
		}
		if view.NutritionIncomplete {
			summary.IncompleteRecordCount++
		}
	}
	summary.Calories = roundNutrition(summary.Calories)
	summary.Carbohydrate = roundNutrition(summary.Carbohydrate)
	summary.Protein = roundNutrition(summary.Protein)
	summary.Fat = roundNutrition(summary.Fat)
	summary.NutritionIncomplete = summary.IncompleteRecordCount > 0

	return MealDayView{
		Date: date.Format("2006-01-02"), Member: member, Summary: summary, Records: views,
	}
}

// MyMealDay lets the current member inspect today or any past date.
func (s *MealService) MyMealDay(ctx context.Context, actor AuthenticatedIdentity, date time.Time) (MealDayView, error) {
	if actor.MemberID == 0 {
		return MealDayView{}, ErrUnauthenticated
	}
	normalized, err := s.normalizeOwnMealDate(date)
	if err != nil {
		return MealDayView{}, err
	}
	records, err := s.repository.ListMealRecordsByMemberAndDate(ctx, repository.ListMealRecordsByMemberAndDateParams{
		MemberID: actor.MemberID, MealDate: normalized,
	})
	if err != nil {
		return MealDayView{}, fmt.Errorf("list my meal day: %w", err)
	}
	member := MealMemberOption{
		ID: actor.MemberID, DisplayName: actor.DisplayName, AvatarKey: actor.AvatarKey, IsCurrent: true,
	}
	return buildMealDayView(member, normalized, records), nil
}

// MyMealCalendar returns only dates that contain records. Missing dates are
// represented by empty cells on the client and do not require placeholder rows.
func (s *MealService) MyMealCalendar(ctx context.Context, actor AuthenticatedIdentity, month string) (MealCalendarView, error) {
	if actor.MemberID == 0 {
		return MealCalendarView{}, ErrUnauthenticated
	}
	start, err := time.ParseInLocation("2006-01", strings.TrimSpace(month), s.location)
	if err != nil {
		return MealCalendarView{}, fmt.Errorf("%w: invalid month", ErrInvalidInput)
	}
	currentMonth := time.Date(s.today().Year(), s.today().Month(), 1, 0, 0, 0, 0, s.location)
	if start.After(currentMonth) {
		return MealCalendarView{}, fmt.Errorf("%w: month cannot be in the future", ErrInvalidInput)
	}
	rows, err := s.repository.ListMealCalendar(ctx, repository.ListMealCalendarParams{
		MemberID: actor.MemberID, StartDate: start, EndDateExclusive: start.AddDate(0, 1, 0),
	})
	if err != nil {
		return MealCalendarView{}, fmt.Errorf("list my meal calendar: %w", err)
	}
	view := MealCalendarView{
		MemberID: actor.MemberID, Month: start.Format("2006-01"), Days: make([]MealCalendarDay, 0, len(rows)),
	}
	for _, row := range rows {
		view.Days = append(view.Days, MealCalendarDay{
			Date: row.MealDate.Format("2006-01-02"), Calories: roundNutrition(row.Calories),
			RecordCount: row.RecordCount, NutritionIncomplete: row.IncompleteRecordCount > 0,
			IncompleteRecordCount: row.IncompleteRecordCount,
		})
	}
	return view, nil
}

// MealMemberOptions returns a privacy-safe active-member list and puts the
// current member first so the page defaults to "my meals".
func (s *MealService) MealMemberOptions(ctx context.Context, actor AuthenticatedIdentity) ([]MealMemberOption, error) {
	if actor.MemberID == 0 {
		return nil, ErrUnauthenticated
	}
	rows, err := s.repository.ListActiveMemberOptions(ctx, actor.HouseholdID)
	if err != nil {
		return nil, fmt.Errorf("list meal member options: %w", err)
	}
	result := make([]MealMemberOption, 0, len(rows))
	for _, row := range rows {
		if row.ID == actor.MemberID {
			result = append(result, MealMemberOption{
				ID: row.ID, DisplayName: row.DisplayName, AvatarKey: row.AvatarKey, IsCurrent: true,
			})
			break
		}
	}
	for _, row := range rows {
		if row.ID != actor.MemberID {
			result = append(result, MealMemberOption{
				ID: row.ID, DisplayName: row.DisplayName, AvatarKey: row.AvatarKey,
			})
		}
	}
	return result, nil
}

// MemberToday accepts no date. The server chooses today's household-local
// date, which prevents crafted requests from reading another member's history.
func (s *MealService) MemberToday(ctx context.Context, actor AuthenticatedIdentity, memberID uint64) (MealDayView, error) {
	if actor.MemberID == 0 {
		return MealDayView{}, ErrUnauthenticated
	}
	if memberID == 0 {
		return MealDayView{}, ErrInvalidInput
	}
	target, err := s.repository.GetMemberByID(ctx, memberID)
	if errors.Is(err, repository.ErrNotFound) {
		return MealDayView{}, ErrNotFound
	}
	if err != nil {
		return MealDayView{}, fmt.Errorf("get meal member: %w", err)
	}
	if target.HouseholdID != actor.HouseholdID || target.Status != model.MemberStatusActive {
		return MealDayView{}, ErrForbidden
	}
	today := s.today()
	records, err := s.repository.ListMealRecordsByMemberAndDate(ctx, repository.ListMealRecordsByMemberAndDateParams{
		MemberID: memberID, MealDate: today,
	})
	if err != nil {
		return MealDayView{}, fmt.Errorf("list member meals today: %w", err)
	}
	member := MealMemberOption{
		ID: memberID, DisplayName: target.DisplayName, AvatarKey: target.AvatarKey,
		IsCurrent: memberID == actor.MemberID,
	}
	return buildMealDayView(member, today, records), nil
}
