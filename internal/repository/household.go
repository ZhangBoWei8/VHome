package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vhome/internal/model"
)

const householdColumns = `
	id,
	login_name,
	display_name,
	join_secret_hash,
	registration_enabled,
	avatar_type,
	avatar_value,
	province,
	city,
	version,
	created_at,
	updated_at
`

type CreateHouseholdParams struct {
	LoginName           string
	DisplayName         string
	JoinSecretHash      string
	RegistrationEnabled bool
	AvatarType          string
	AvatarValue         string
}

type UpdateHouseholdParams struct {
	ID          uint64
	DisplayName string
	Province    string
	City        string
	Version     uint64
}

// 该接口的作用是让传入的参数不需要考虑是哪一种，只需要是实现了Scan方法的结构的对象即可，因为sql中多类型都实现了Scan()函数
type householdScanner interface {
	Scan(dest ...any) error
}

// 单纯提取出来全量读household的方法
func scanHousehold(scanner householdScanner) (model.Household, error) {
	var household model.Household

	err := scanner.Scan(
		&household.ID,
		&household.LoginName,
		&household.DisplayName,
		&household.JoinSecretHash,
		&household.RegistrationEnabled,
		&household.AvatarType,
		&household.AvatarValue,
		&household.Province,
		&household.City,
		&household.Version,
		&household.CreatedAt,
		&household.UpdatedAt,
	)
	if err != nil {
		return model.Household{}, err
	}

	return household, nil
}

// 判断是否已经存在家庭
func (r *Repository) HouseholdExists(ctx context.Context) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM households
			WHERE singleton_key = 1
		)
	`

	var exists bool

	if err := r.q.QueryRowContext(
		ctx,
		query,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf(
			"check household existence: %w",
			err,
		)
	}

	return exists, nil
}

// 获取家庭信息
func (r *Repository) GetHousehold(ctx context.Context) (model.Household, error) {
	query := `
		SELECT ` + householdColumns + `
		FROM households
		WHERE singleton_key = 1
		LIMIT 1
	`

	household, err := scanHousehold(
		r.q.QueryRowContext(ctx, query),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Household{}, ErrNotFound
	}
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"get household: %w",
			err,
		)
	}

	return household, nil
}

func (r *Repository) GetHouseholdByID(ctx context.Context, id uint64) (model.Household, error) {
	query := `
		SELECT ` + householdColumns + `
		FROM households
		WHERE id = ?
		LIMIT 1
	`

	household, err := scanHousehold(
		r.q.QueryRowContext(ctx, query, id),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Household{}, ErrNotFound
	}
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"get household by id: %w",
			err,
		)
	}

	return household, nil
}

func (r *Repository) CreateHousehold(ctx context.Context, params CreateHouseholdParams) (model.Household, error) {
	const query = `
		INSERT INTO households (
			singleton_key,
			login_name,
			display_name,
			join_secret_hash,
			registration_enabled,
			avatar_type,
			avatar_value
		)
		VALUES (1, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.LoginName,
		params.DisplayName,
		params.JoinSecretHash,
		params.RegistrationEnabled,
		params.AvatarType,
		params.AvatarValue,
	)
	if isDuplicateKey(err) {
		return model.Household{}, ErrConflict
	}
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"create household: %w",
			err,
		)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"get created household id: %w",
			err,
		)
	}

	return r.GetHouseholdByID(ctx, uint64(id))
}

func (r *Repository) UpdateHousehold(ctx context.Context, params UpdateHouseholdParams) (model.Household, error) {
	const query = `
		UPDATE households
		SET
			display_name = ?,
			province = ?,
			city = ?,
			version = version + 1
		WHERE id = ?
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.DisplayName,
		params.Province,
		params.City,
		params.ID,
		params.Version,
	)
	if isDuplicateKey(err) {
		return model.Household{}, ErrConflict
	}
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"update household: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Household{}, fmt.Errorf(
			"get updated household rows: %w",
			err,
		)
	}

	if affected == 0 {
		return model.Household{}, ErrConflict
	}

	return r.GetHouseholdByID(ctx, params.ID)
}

func (r *Repository) UpdateHouseholdJoinSecret(ctx context.Context, householdID uint64, joinSecretHash string) error {
	const query = `
		UPDATE households
		SET
			join_secret_hash = ?,
			version = version + 1
		WHERE id = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		joinSecretHash,
		householdID,
	)
	if err != nil {
		return fmt.Errorf(
			"update household join secret: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get join secret update rows: %w",
			err,
		)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}
