package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vhome/internal/model"
)

type CreateLocationParams struct {
	Name, IconKey string
	StorageType   model.StorageType
}
type CreateTemplateParams struct {
	Name, IconKey, DefaultUnit    string
	ColdDays, AmbientDays         *int
	Calories, Protein, Fat, Carbs *float64
}
type CreateInventoryParams struct {
	TemplateID                    *uint64
	LocationID                    uint64
	Name, IconKey                 string
	Quantity                      float64
	Unit                          string
	StockedOn, ExpiresOn          time.Time
	Calories, Protein, Fat, Carbs *float64
	Description, ImagePath        string
	CreatedBy                     uint64
}
type UpdateInventoryParams struct {
	ID, LocationID                uint64
	Name, IconKey                 string
	Quantity                      float64
	Unit                          string
	StockedOn, ExpiresOn          time.Time
	Calories, Protein, Fat, Carbs *float64
	Description, ImagePath        string
	Version                       uint64
}

func nullableValue[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullableText(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (r *Repository) ListStorageLocations(ctx context.Context) ([]model.StorageLocation, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id,name,icon_key,storage_type,is_builtin,enabled,created_at FROM storage_locations WHERE enabled=TRUE ORDER BY is_builtin DESC,id`)
	if err != nil {
		return nil, fmt.Errorf("list storage locations: %w", err)
	}
	defer rows.Close()
	result := make([]model.StorageLocation, 0)
	for rows.Next() {
		var v model.StorageLocation
		if err := rows.Scan(&v.ID, &v.Name, &v.IconKey, &v.StorageType, &v.IsBuiltin, &v.Enabled, &v.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *Repository) GetStorageLocation(ctx context.Context, id uint64) (model.StorageLocation, error) {
	var v model.StorageLocation
	err := r.q.QueryRowContext(ctx, `SELECT id,name,icon_key,storage_type,is_builtin,enabled,created_at FROM storage_locations WHERE id=?`, id).Scan(&v.ID, &v.Name, &v.IconKey, &v.StorageType, &v.IsBuiltin, &v.Enabled, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, fmt.Errorf("get location: %w", err)
	}
	return v, nil
}

func (r *Repository) CreateStorageLocation(ctx context.Context, p CreateLocationParams) (model.StorageLocation, error) {
	res, err := r.q.ExecContext(ctx, `INSERT INTO storage_locations(name,icon_key,storage_type) VALUES(?,?,?)`, p.Name, p.IconKey, p.StorageType)
	if isDuplicateKey(err) {
		return model.StorageLocation{}, ErrConflict
	}
	if err != nil {
		return model.StorageLocation{}, err
	}
	id, _ := res.LastInsertId()
	return r.GetStorageLocation(ctx, uint64(id))
}

func scanTemplate(s interface{ Scan(...any) error }) (model.MaterialTemplate, error) {
	var v model.MaterialTemplate
	var cold, ambient sql.NullInt64
	var cal, pro, fat, carb sql.NullFloat64
	err := s.Scan(&v.ID, &v.Name, &v.IconKey, &v.DefaultUnit, &cold, &ambient, &cal, &pro, &fat, &carb, &v.Source, &v.Enabled, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	if cold.Valid {
		x := int(cold.Int64)
		v.ColdShelfLifeDays = &x
	}
	if ambient.Valid {
		x := int(ambient.Int64)
		v.AmbientShelfLifeDays = &x
	}
	if cal.Valid {
		x := cal.Float64
		v.CaloriesPer100G = &x
	}
	if pro.Valid {
		x := pro.Float64
		v.ProteinPer100G = &x
	}
	if fat.Valid {
		x := fat.Float64
		v.FatPer100G = &x
	}
	if carb.Valid {
		x := carb.Float64
		v.CarbohydratePer100G = &x
	}
	return v, nil
}

const templateCols = `id,name,icon_key,default_unit,cold_shelf_life_days,ambient_shelf_life_days,calories_per_100g,protein_per_100g,fat_per_100g,carbohydrate_per_100g,source,enabled,version,created_at,updated_at`

func (r *Repository) ListMaterialTemplates(ctx context.Context) ([]model.MaterialTemplate, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+templateCols+` FROM material_templates WHERE enabled=TRUE ORDER BY source,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.MaterialTemplate, 0)
	for rows.Next() {
		v, e := scanTemplate(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) GetMaterialTemplate(ctx context.Context, id uint64) (model.MaterialTemplate, error) {
	v, e := scanTemplate(r.q.QueryRowContext(ctx, `SELECT `+templateCols+` FROM material_templates WHERE id=? AND enabled=TRUE`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *Repository) CreateMaterialTemplate(ctx context.Context, p CreateTemplateParams) (model.MaterialTemplate, error) {
	res, e := r.q.ExecContext(ctx, `INSERT INTO material_templates(name,icon_key,default_unit,cold_shelf_life_days,ambient_shelf_life_days,calories_per_100g,protein_per_100g,fat_per_100g,carbohydrate_per_100g,source) VALUES(?,?,?,?,?,?,?,?,?,'USER')`, p.Name, p.IconKey, p.DefaultUnit, nullableValue(p.ColdDays), nullableValue(p.AmbientDays), nullableValue(p.Calories), nullableValue(p.Protein), nullableValue(p.Fat), nullableValue(p.Carbs))
	if isDuplicateKey(e) {
		return model.MaterialTemplate{}, ErrConflict
	}
	if e != nil {
		return model.MaterialTemplate{}, e
	}
	id, _ := res.LastInsertId()
	return r.GetMaterialTemplate(ctx, uint64(id))
}

const inventoryCols = `i.id,i.template_id,i.storage_location_id,i.name,i.icon_key,i.quantity,i.unit,i.stocked_on,i.expires_on,i.calories_per_100g,i.protein_per_100g,i.fat_per_100g,i.carbohydrate_per_100g,i.description,i.image_path,i.status,i.discarded_at,i.discarded_by,i.discard_reason,i.version,i.created_by,i.created_at,i.updated_at,l.name,l.storage_type`

func scanInventory(s interface{ Scan(...any) error }) (model.InventoryItem, error) {
	var v model.InventoryItem
	var tid, db sql.NullInt64
	var cal, pro, fat, carb sql.NullFloat64
	var desc, img, reason sql.NullString
	var da sql.NullTime
	e := s.Scan(&v.ID, &tid, &v.StorageLocationID, &v.Name, &v.IconKey, &v.Quantity, &v.Unit, &v.StockedOn, &v.ExpiresOn, &cal, &pro, &fat, &carb, &desc, &img, &v.Status, &da, &db, &reason, &v.Version, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.LocationName, &v.StorageType)
	if e != nil {
		return v, e
	}
	if tid.Valid {
		x := uint64(tid.Int64)
		v.TemplateID = &x
	}
	if cal.Valid {
		x := cal.Float64
		v.CaloriesPer100G = &x
	}
	if pro.Valid {
		x := pro.Float64
		v.ProteinPer100G = &x
	}
	if fat.Valid {
		x := fat.Float64
		v.FatPer100G = &x
	}
	if carb.Valid {
		x := carb.Float64
		v.CarbohydratePer100G = &x
	}
	if desc.Valid {
		v.Description = desc.String
	}
	if img.Valid {
		v.ImagePath = img.String
	}
	if da.Valid {
		x := da.Time
		v.DiscardedAt = &x
	}
	if db.Valid {
		x := uint64(db.Int64)
		v.DiscardedBy = &x
	}
	if reason.Valid {
		v.DiscardReason = reason.String
	}
	return v, nil
}
func (r *Repository) ListInventory(ctx context.Context, scope, order string) ([]model.InventoryItem, error) {
	// Inventory dates are household-local calendar dates. VHome currently uses
	// Asia/Shanghai, so do not let a UTC database session move an item between
	// "active" and "expired" eight hours too early.
	today := `DATE(DATE_ADD(UTC_TIMESTAMP(), INTERVAL 8 HOUR))`
	where := `i.status='ACTIVE' AND i.expires_on>=` + today
	if scope == "expired" {
		where = `i.status='ACTIVE' AND i.expires_on<` + today
	}
	direction := "ASC"
	if order == "desc" {
		direction = "DESC"
	}
	rows, e := r.q.QueryContext(ctx, `SELECT `+inventoryCols+` FROM inventory_items i JOIN storage_locations l ON l.id=i.storage_location_id WHERE `+where+` ORDER BY i.expires_on `+direction+`,i.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]model.InventoryItem, 0)
	for rows.Next() {
		v, e := scanInventory(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) GetInventory(ctx context.Context, id uint64) (model.InventoryItem, error) {
	v, e := scanInventory(r.q.QueryRowContext(ctx, `SELECT `+inventoryCols+` FROM inventory_items i JOIN storage_locations l ON l.id=i.storage_location_id WHERE i.id=?`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *Repository) CreateInventory(ctx context.Context, p CreateInventoryParams) (model.InventoryItem, error) {
	res, e := r.q.ExecContext(ctx, `INSERT INTO inventory_items(template_id,storage_location_id,name,icon_key,quantity,unit,stocked_on,expires_on,calories_per_100g,protein_per_100g,fat_per_100g,carbohydrate_per_100g,description,image_path,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, nullableValue(p.TemplateID), p.LocationID, p.Name, p.IconKey, p.Quantity, p.Unit, p.StockedOn, p.ExpiresOn, nullableValue(p.Calories), nullableValue(p.Protein), nullableValue(p.Fat), nullableValue(p.Carbs), nullableText(p.Description), nullableText(p.ImagePath), p.CreatedBy)
	if e != nil {
		return model.InventoryItem{}, e
	}
	id, _ := res.LastInsertId()
	return r.GetInventory(ctx, uint64(id))
}
func (r *Repository) UpdateInventory(ctx context.Context, p UpdateInventoryParams) (model.InventoryItem, error) {
	res, e := r.q.ExecContext(ctx, `UPDATE inventory_items SET storage_location_id=?,name=?,icon_key=?,quantity=?,unit=?,stocked_on=?,expires_on=?,calories_per_100g=?,protein_per_100g=?,fat_per_100g=?,carbohydrate_per_100g=?,description=?,image_path=?,version=version+1 WHERE id=? AND status='ACTIVE' AND version=?`, p.LocationID, p.Name, p.IconKey, p.Quantity, p.Unit, p.StockedOn, p.ExpiresOn, nullableValue(p.Calories), nullableValue(p.Protein), nullableValue(p.Fat), nullableValue(p.Carbs), nullableText(p.Description), nullableText(p.ImagePath), p.ID, p.Version)
	if e != nil {
		return model.InventoryItem{}, e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.InventoryItem{}, ErrConflict
	}
	return r.GetInventory(ctx, p.ID)
}
func (r *Repository) DiscardInventory(ctx context.Context, id, memberID, version uint64, reason string) error {
	res, e := r.q.ExecContext(ctx, `UPDATE inventory_items SET status='DISCARDED',discarded_at=UTC_TIMESTAMP(6),discarded_by=?,discard_reason=?,version=version+1 WHERE id=? AND status='ACTIVE' AND version=?`, memberID, nullableText(reason), id, version)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (r *Repository) ReminderRead(ctx context.Context, memberID, itemID uint64, m model.ReminderMilestone) (bool, error) {
	var n int
	e := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_reminder_reads WHERE member_id=? AND item_id=? AND milestone=?`, memberID, itemID, m).Scan(&n)
	return n > 0, e
}
func (r *Repository) MarkReminderRead(ctx context.Context, memberID, itemID uint64, m model.ReminderMilestone) error {
	_, e := r.q.ExecContext(ctx, `INSERT IGNORE INTO material_reminder_reads(member_id,item_id,milestone) VALUES(?,?,?)`, memberID, itemID, m)
	return e
}
