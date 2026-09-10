package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vhome/internal/model"
)

const foodColumns = `
	id,
	name,
	calories_per_100g,
	carbohydrate_per_100g,
	protein_per_100g,
	fat_per_100g,
	icon_type,
	icon_value,
	source,
	created_by,
	deleted_by,
	version,
	deleted_at,
	created_at,
	updated_at
`

type CreateFoodParams struct {
	Name                string
	CaloriesPer100G     float64
	CarbohydratePer100G *float64
	ProteinPer100G      *float64
	FatPer100G          *float64
	IconType            model.FoodIconType
	IconValue           string
	Source              model.FoodSource
	CreatedBy           *uint64
}

type UpdateFoodParams struct {
	ID                  uint64
	Name                string
	CaloriesPer100G     float64
	CarbohydratePer100G *float64
	ProteinPer100G      *float64
	FatPer100G          *float64
	IconType            model.FoodIconType
	IconValue           string
	Version             uint64
}

type SoftDeleteFoodParams struct {
	ID        uint64
	DeletedBy uint64
	Version   uint64
}

type RestoreFoodParams struct {
	ID      uint64
	Version uint64
}

type CreateMealRecordParams struct {
	MemberID                    uint64
	MealDate                    time.Time
	MealType                    model.MealType
	FoodID                      *uint64
	FoodNameSnapshot            string
	IconTypeSnapshot            model.FoodIconType
	IconValueSnapshot           string
	WeightGrams                 float64
	CaloriesPer100GSnapshot     float64
	CarbohydratePer100GSnapshot *float64
	ProteinPer100GSnapshot      *float64
	FatPer100GSnapshot          *float64
	CreatedBy                   uint64
}

type UpdateMealRecordParams struct {
	ID                          uint64
	MemberID                    uint64
	MealDate                    time.Time
	MealType                    model.MealType
	FoodID                      *uint64
	FoodNameSnapshot            string
	IconTypeSnapshot            model.FoodIconType
	IconValueSnapshot           string
	WeightGrams                 float64
	CaloriesPer100GSnapshot     float64
	CarbohydratePer100GSnapshot *float64
	ProteinPer100GSnapshot      *float64
	FatPer100GSnapshot          *float64
	Version                     uint64
}

type SoftDeleteMealRecordParams struct {
	ID        uint64
	MemberID  uint64
	DeletedBy uint64
	Version   uint64
}

type ListMealRecordsByMemberAndDateParams struct {
	MemberID uint64
	MealDate time.Time
}

type ListMealCalendarParams struct {
	MemberID         uint64
	StartDate        time.Time
	EndDateExclusive time.Time
}

type MealCalendarRow struct {
	MealDate              time.Time
	Calories              float64
	RecordCount           uint64
	IncompleteRecordCount uint64
}

const mealRecordColumns = `
	id,
	member_id,
	meal_date,
	meal_type,
	food_id,
	food_name_snapshot,
	icon_type_snapshot,
	icon_value_snapshot,
	weight_grams,
	calories_per_100g_snapshot,
	carbohydrate_per_100g_snapshot,
	protein_per_100g_snapshot,
	fat_per_100g_snapshot,
	created_by,
	deleted_by,
	version,
	deleted_at,
	created_at,
	updated_at
`

type foodScanner interface {
	Scan(dest ...any) error
}

func scanFood(scanner foodScanner) (model.Food, error) {
	var food model.Food

	var carbohydrate sql.NullFloat64
	var protein sql.NullFloat64
	var fat sql.NullFloat64
	var createdBy sql.NullInt64
	var deletedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&food.ID,
		&food.Name,
		&food.CaloriesPer100G,
		&carbohydrate,
		&protein,
		&fat,
		&food.IconType,
		&food.IconValue,
		&food.Source,
		&createdBy,
		&deletedBy,
		&food.Version,
		&deletedAt,
		&food.CreatedAt,
		&food.UpdatedAt,
	)
	if err != nil {
		return model.Food{}, err
	}

	if carbohydrate.Valid {
		value := carbohydrate.Float64
		food.CarbohydratePer100G = &value
	}

	if protein.Valid {
		value := protein.Float64
		food.ProteinPer100G = &value
	}

	if fat.Valid {
		value := fat.Float64
		food.FatPer100G = &value
	}

	if createdBy.Valid {
		value := uint64(createdBy.Int64)
		food.CreatedBy = &value
	}

	if deletedBy.Valid {
		value := uint64(deletedBy.Int64)
		food.DeletedBy = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		food.DeletedAt = &value
	}

	return food, nil
}

func (r *Repository) GetFoodByName(ctx context.Context, name string) (model.Food, error) {
	query := `
		SELECT ` + foodColumns + `
		FROM foods
		WHERE name = ?
		LIMIT 1
	`

	food, err := scanFood(
		r.q.QueryRowContext(ctx, query, name),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Food{}, ErrNotFound
	}
	if err != nil {
		return model.Food{}, fmt.Errorf(
			"get food by name: %w",
			err,
		)
	}

	return food, nil
}

func (r *Repository) GetFoodByID(ctx context.Context, foodID uint64) (model.Food, error) {
	query := `
		SELECT ` + foodColumns + `
		FROM foods
		WHERE id = ?
		LIMIT 1
	`

	food, err := scanFood(r.q.QueryRowContext(ctx, query, foodID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Food{}, ErrNotFound
	}
	if err != nil {
		return model.Food{}, fmt.Errorf("get food by id: %w", err)
	}

	return food, nil
}

func (r *Repository) GetActiveFoodByID(ctx context.Context, foodID uint64) (model.Food, error) {
	query := `
		SELECT ` + foodColumns + `
		FROM foods
		WHERE id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	food, err := scanFood(r.q.QueryRowContext(ctx, query, foodID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Food{}, ErrNotFound
	}
	if err != nil {
		return model.Food{}, fmt.Errorf("get active food by id: %w", err)
	}

	return food, nil
}

func (r *Repository) ListActiveFoods(ctx context.Context, keyword string) ([]model.Food, error) {
	return r.listFoods(ctx, keyword, false)
}

func (r *Repository) ListDeletedFoods(ctx context.Context, keyword string) ([]model.Food, error) {
	return r.listFoods(ctx, keyword, true)
}

func (r *Repository) listFoods(ctx context.Context, keyword string, deleted bool) ([]model.Food, error) {
	deletedCondition := "deleted_at IS NULL"
	if deleted {
		deletedCondition = "deleted_at IS NOT NULL"
	}

	query := `
		SELECT ` + foodColumns + `
		FROM foods
		WHERE ` + deletedCondition + `
		  AND (? = '' OR name LIKE CONCAT('%', ?, '%'))
		ORDER BY source ASC, name ASC, id ASC
	`

	rows, err := r.q.QueryContext(ctx, query, keyword, keyword)
	if err != nil {
		return nil, fmt.Errorf("list foods: %w", err)
	}
	defer rows.Close()

	foods := make([]model.Food, 0)
	for rows.Next() {
		food, err := scanFood(rows)
		if err != nil {
			return nil, fmt.Errorf("scan food list row: %w", err)
		}
		foods = append(foods, food)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate food list: %w", err)
	}

	return foods, nil
}

func (r *Repository) CreateFood(ctx context.Context, params CreateFoodParams) (model.Food, error) {
	const query = `
		INSERT INTO foods (
			name,
			calories_per_100g,
			carbohydrate_per_100g,
			protein_per_100g,
			fat_per_100g,
			icon_type,
			icon_value,
			source,
			created_by
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.Name,
		params.CaloriesPer100G,
		nullableValue(params.CarbohydratePer100G),
		nullableValue(params.ProteinPer100G),
		nullableValue(params.FatPer100G),
		string(params.IconType),
		params.IconValue,
		string(params.Source),
		nullableValue(params.CreatedBy),
	)
	if isDuplicateKey(err) {
		return model.Food{}, ErrConflict
	}
	if err != nil {
		return model.Food{}, fmt.Errorf("create food: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Food{}, fmt.Errorf("get created food id: %w", err)
	}

	return r.GetFoodByID(ctx, uint64(id))
}

func (r *Repository) UpdateFood(ctx context.Context, params UpdateFoodParams) (model.Food, error) {
	const query = `
		UPDATE foods
		SET name = ?,
			calories_per_100g = ?,
			carbohydrate_per_100g = ?,
			protein_per_100g = ?,
			fat_per_100g = ?,
			icon_type = ?,
			icon_value = ?,
			version = version + 1
		WHERE id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.Name,
		params.CaloriesPer100G,
		nullableValue(params.CarbohydratePer100G),
		nullableValue(params.ProteinPer100G),
		nullableValue(params.FatPer100G),
		string(params.IconType),
		params.IconValue,
		params.ID,
		params.Version,
	)
	if isDuplicateKey(err) {
		return model.Food{}, ErrConflict
	}
	if err != nil {
		return model.Food{}, fmt.Errorf("update food: %w", err)
	}

	if err := requireOneRowAffected(result); err != nil {
		return model.Food{}, err
	}

	return r.GetFoodByID(ctx, params.ID)
}

func (r *Repository) SoftDeleteFood(ctx context.Context, params SoftDeleteFoodParams) (model.Food, error) {
	const query = `
		UPDATE foods
		SET deleted_at = UTC_TIMESTAMP(6),
			deleted_by = ?,
			version = version + 1
		WHERE id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(ctx, query, params.DeletedBy, params.ID, params.Version)
	if err != nil {
		return model.Food{}, fmt.Errorf("soft delete food: %w", err)
	}
	if err := requireOneRowAffected(result); err != nil {
		return model.Food{}, err
	}

	return r.GetFoodByID(ctx, params.ID)
}

func (r *Repository) RestoreFood(ctx context.Context, params RestoreFoodParams) (model.Food, error) {
	const query = `
		UPDATE foods
		SET deleted_at = NULL,
			deleted_by = NULL,
			version = version + 1
		WHERE id = ?
		  AND deleted_at IS NOT NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(ctx, query, params.ID, params.Version)
	if err != nil {
		return model.Food{}, fmt.Errorf("restore food: %w", err)
	}
	if err := requireOneRowAffected(result); err != nil {
		return model.Food{}, err
	}

	return r.GetFoodByID(ctx, params.ID)
}

type mealRecordScanner interface {
	Scan(dest ...any) error
}

func scanMealRecord(scanner mealRecordScanner) (model.MealRecord, error) {
	var record model.MealRecord

	var foodID sql.NullInt64
	var carbohydrate sql.NullFloat64
	var protein sql.NullFloat64
	var fat sql.NullFloat64
	var deletedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&record.ID,
		&record.MemberID,
		&record.MealDate,
		&record.MealType,
		&foodID,
		&record.FoodNameSnapshot,
		&record.IconTypeSnapshot,
		&record.IconValueSnapshot,
		&record.WeightGrams,
		&record.CaloriesPer100GSnapshot,
		&carbohydrate,
		&protein,
		&fat,
		&record.CreatedBy,
		&deletedBy,
		&record.Version,
		&deletedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return model.MealRecord{}, err
	}

	if foodID.Valid {
		value := uint64(foodID.Int64)
		record.FoodID = &value
	}
	if carbohydrate.Valid {
		value := carbohydrate.Float64
		record.CarbohydratePer100GSnapshot = &value
	}
	if protein.Valid {
		value := protein.Float64
		record.ProteinPer100GSnapshot = &value
	}
	if fat.Valid {
		value := fat.Float64
		record.FatPer100GSnapshot = &value
	}
	if deletedBy.Valid {
		value := uint64(deletedBy.Int64)
		record.DeletedBy = &value
	}
	if deletedAt.Valid {
		value := deletedAt.Time
		record.DeletedAt = &value
	}

	return record, nil
}

func (r *Repository) GetActiveMealRecordByID(ctx context.Context, recordID, memberID uint64) (model.MealRecord, error) {
	query := `
		SELECT ` + mealRecordColumns + `
		FROM meal_records
		WHERE id = ?
		  AND member_id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	record, err := scanMealRecord(r.q.QueryRowContext(ctx, query, recordID, memberID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.MealRecord{}, ErrNotFound
	}
	if err != nil {
		return model.MealRecord{}, fmt.Errorf("get active meal record by id: %w", err)
	}

	return record, nil
}

func (r *Repository) CreateMealRecord(ctx context.Context, params CreateMealRecordParams) (model.MealRecord, error) {
	const query = `
		INSERT INTO meal_records (
			member_id,
			meal_date,
			meal_type,
			food_id,
			food_name_snapshot,
			icon_type_snapshot,
			icon_value_snapshot,
			weight_grams,
			calories_per_100g_snapshot,
			carbohydrate_per_100g_snapshot,
			protein_per_100g_snapshot,
			fat_per_100g_snapshot,
			created_by
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.MemberID,
		params.MealDate,
		string(params.MealType),
		nullableValue(params.FoodID),
		params.FoodNameSnapshot,
		string(params.IconTypeSnapshot),
		params.IconValueSnapshot,
		params.WeightGrams,
		params.CaloriesPer100GSnapshot,
		nullableValue(params.CarbohydratePer100GSnapshot),
		nullableValue(params.ProteinPer100GSnapshot),
		nullableValue(params.FatPer100GSnapshot),
		params.CreatedBy,
	)
	if err != nil {
		return model.MealRecord{}, fmt.Errorf("create meal record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.MealRecord{}, fmt.Errorf("get created meal record id: %w", err)
	}

	return r.GetActiveMealRecordByID(ctx, uint64(id), params.MemberID)
}

func (r *Repository) UpdateMealRecord(ctx context.Context, params UpdateMealRecordParams) (model.MealRecord, error) {
	const query = `
		UPDATE meal_records
		SET meal_date = ?,
			meal_type = ?,
			food_id = ?,
			food_name_snapshot = ?,
			icon_type_snapshot = ?,
			icon_value_snapshot = ?,
			weight_grams = ?,
			calories_per_100g_snapshot = ?,
			carbohydrate_per_100g_snapshot = ?,
			protein_per_100g_snapshot = ?,
			fat_per_100g_snapshot = ?,
			version = version + 1
		WHERE id = ?
		  AND member_id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.MealDate,
		string(params.MealType),
		nullableValue(params.FoodID),
		params.FoodNameSnapshot,
		string(params.IconTypeSnapshot),
		params.IconValueSnapshot,
		params.WeightGrams,
		params.CaloriesPer100GSnapshot,
		nullableValue(params.CarbohydratePer100GSnapshot),
		nullableValue(params.ProteinPer100GSnapshot),
		nullableValue(params.FatPer100GSnapshot),
		params.ID,
		params.MemberID,
		params.Version,
	)
	if err != nil {
		return model.MealRecord{}, fmt.Errorf("update meal record: %w", err)
	}
	if err := requireOneRowAffected(result); err != nil {
		return model.MealRecord{}, err
	}

	return r.GetActiveMealRecordByID(ctx, params.ID, params.MemberID)
}

func (r *Repository) SoftDeleteMealRecord(ctx context.Context, params SoftDeleteMealRecordParams) error {
	const query = `
		UPDATE meal_records
		SET deleted_at = UTC_TIMESTAMP(6),
			deleted_by = ?,
			version = version + 1
		WHERE id = ?
		  AND member_id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.DeletedBy,
		params.ID,
		params.MemberID,
		params.Version,
	)
	if err != nil {
		return fmt.Errorf("soft delete meal record: %w", err)
	}

	return requireOneRowAffected(result)
}

func (r *Repository) ListMealRecordsByMemberAndDate(ctx context.Context, params ListMealRecordsByMemberAndDateParams) ([]model.MealRecord, error) {
	query := `
		SELECT ` + mealRecordColumns + `
		FROM meal_records
		WHERE member_id = ?
		  AND meal_date = ?
		  AND deleted_at IS NULL
		ORDER BY FIELD(meal_type, 'BREAKFAST', 'LUNCH', 'DINNER', 'SNACK'),
			created_at ASC,
			id ASC
	`

	rows, err := r.q.QueryContext(ctx, query, params.MemberID, params.MealDate)
	if err != nil {
		return nil, fmt.Errorf("list meal records by member and date: %w", err)
	}
	defer rows.Close()

	records := make([]model.MealRecord, 0)
	for rows.Next() {
		record, err := scanMealRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan meal record list row: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meal record list: %w", err)
	}

	return records, nil
}

func (r *Repository) ListMealCalendar(ctx context.Context, params ListMealCalendarParams) ([]MealCalendarRow, error) {
	const query = `
		SELECT meal_date,
			COALESCE(SUM(weight_grams / 100 * calories_per_100g_snapshot), 0),
			COUNT(*),
			SUM(
				CASE
					WHEN carbohydrate_per_100g_snapshot IS NULL
					  OR protein_per_100g_snapshot IS NULL
					  OR fat_per_100g_snapshot IS NULL
					THEN 1
					ELSE 0
				END
			)
		FROM meal_records
		WHERE member_id = ?
		  AND meal_date >= ?
		  AND meal_date < ?
		  AND deleted_at IS NULL
		GROUP BY meal_date
		ORDER BY meal_date ASC
	`

	rows, err := r.q.QueryContext(
		ctx,
		query,
		params.MemberID,
		params.StartDate,
		params.EndDateExclusive,
	)
	if err != nil {
		return nil, fmt.Errorf("list meal calendar: %w", err)
	}
	defer rows.Close()

	days := make([]MealCalendarRow, 0)
	for rows.Next() {
		var day MealCalendarRow
		if err := rows.Scan(
			&day.MealDate,
			&day.Calories,
			&day.RecordCount,
			&day.IncompleteRecordCount,
		); err != nil {
			return nil, fmt.Errorf("scan meal calendar row: %w", err)
		}
		days = append(days, day)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meal calendar: %w", err)
	}

	return days, nil
}

func requireOneRowAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected row count: %w", err)
	}
	if rowsAffected != 1 {
		return ErrConflict
	}

	return nil
}
