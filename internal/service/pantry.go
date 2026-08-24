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

const (
	maxInventorySearchNameLength  = 128
	maxMaterialTemplateNameLength = 64
	defaultInventoryQueryLimit    = 20
	maxInventoryQueryLimit        = 50
	maxInventoryWithinDays        = 365
)

type PantryService struct {
	repository *repository.Repository
}

func NewPantryService(r *repository.Repository) *PantryService {
	return &PantryService{repository: r}
}

type CreateTemplateInput struct {
	Name, IconKey, DefaultUnit    string
	ColdDays, AmbientDays         *int
	Calories, Protein, Fat, Carbs *float64
}

type InventoryInput struct {
	TemplateID                    *uint64
	LocationID                    uint64
	Name, IconKey                 string
	Quantity                      float64
	Unit                          string
	StockedOn, ExpiresOn          time.Time
	Calories, Protein, Fat, Carbs *float64
	Description, ImagePath        string
}

type InventoryView struct {
	model.InventoryItem
	RemainingDays int `json:"remaining_days"`
	TotalDays     int `json:"total_days"`
}

type MaterialReminder struct {
	ItemID        uint64                  `json:"item_id"`
	ItemName      string                  `json:"item_name"`
	Milestone     model.ReminderMilestone `json:"milestone"`
	RemainingDays int                     `json:"remaining_days"`
	Message       string                  `json:"message"`
}

type NotificationSummary struct {
	PendingMembers    uint64             `json:"pending_members"`
	MaterialReminders []MaterialReminder `json:"material_reminders"`
}

func canManagePantry(a AuthenticatedIdentity) bool {
	return a.Role == model.MemberRoleOwner || a.Role == model.MemberRoleAdmin
}

func validateTemplateNutrition(input CreateTemplateInput) error {
	if input.Calories != nil {
		if err := validateNutritionNumber("calories_per_100g", *input.Calories, 0, 1000); err != nil {
			return err
		}
	}

	if err := validateOptionalNutritionNumber("protein_per_100g", input.Protein, 0, 100); err != nil {
		return err
	}

	if err := validateOptionalNutritionNumber("fat_per_100g", input.Fat, 0, 100); err != nil {
		return err
	}

	if err := validateOptionalNutritionNumber("carbohydrate_per_100g", input.Carbs, 0, 100); err != nil {
		return err
	}

	return nil
}

func (s *PantryService) Locations(ctx context.Context) ([]model.StorageLocation, error) {
	return s.repository.ListStorageLocations(ctx)
}

func (s *PantryService) CreateLocation(ctx context.Context, a AuthenticatedIdentity, name, icon string, t model.StorageType) (model.StorageLocation, error) {
	if !canManagePantry(a) {
		return model.StorageLocation{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" || (t != model.StorageTypeCold && t != model.StorageTypeAmbient) {
		return model.StorageLocation{}, ErrInvalidInput
	}
	if icon == "" {
		icon = "generic-storage"
	}
	v, e := s.repository.CreateStorageLocation(ctx, repository.CreateLocationParams{Name: name, IconKey: icon, StorageType: t})
	if errors.Is(e, repository.ErrConflict) {
		return v, ErrConflict
	}
	return v, e
}

func (s *PantryService) Templates(ctx context.Context) ([]model.MaterialTemplate, error) {
	return s.repository.ListMaterialTemplates(ctx)
}

func (s *PantryService) MaterialTemplateByName(ctx context.Context, name string) (model.MaterialTemplate, error) {
	name = strings.TrimSpace(name)

	if name == "" || len([]rune(name)) > maxMaterialTemplateNameLength {
		return model.MaterialTemplate{}, ErrInvalidInput
	}

	template, err := s.repository.GetMaterialTemplateByName(
		ctx,
		name,
	)
	if errors.Is(err, repository.ErrNotFound) {
		return model.MaterialTemplate{}, ErrNotFound
	}
	if err != nil {
		return model.MaterialTemplate{}, fmt.Errorf(
			"get material template by name: %w",
			err,
		)
	}

	return template, nil
}

func (s *PantryService) CreateTemplate(ctx context.Context, actor AuthenticatedIdentity, input CreateTemplateInput) (model.MaterialTemplate, error) {
	if !canManagePantry(actor) {
		return model.MaterialTemplate{}, ErrForbidden
	}

	input.Name = strings.TrimSpace(input.Name)
	input.IconKey = strings.TrimSpace(input.IconKey)
	input.DefaultUnit = strings.TrimSpace(input.DefaultUnit)

	if input.Name == "" || (input.ColdDays == nil && input.AmbientDays == nil) {
		return model.MaterialTemplate{}, ErrInvalidInput
	}

	if err := validateTemplateNutrition(input); err != nil {
		return model.MaterialTemplate{}, err
	}

	if input.IconKey == "" {
		input.IconKey = "generic-food"
	}

	if input.DefaultUnit == "" {
		input.DefaultUnit = "G"
	}

	var createdTemplate model.MaterialTemplate

	err := s.repository.WithinTransaction(
		ctx,
		func(txRepository *repository.Repository) error {
			var err error

			// First create the reusable material template. If any later database
			// operation fails, the surrounding transaction rolls it back.
			createdTemplate, err = txRepository.CreateMaterialTemplate(
				ctx,
				repository.CreateTemplateParams{
					Name:        input.Name,
					IconKey:     input.IconKey,
					DefaultUnit: input.DefaultUnit,
					ColdDays:    input.ColdDays,
					AmbientDays: input.AmbientDays,
					Calories:    input.Calories,
					Protein:     input.Protein,
					Fat:         input.Fat,
					Carbs:       input.Carbs,
				},
			)
			if err != nil {
				return err
			}

			// foods requires calories. A material without calories remains a
			// pantry-only template instead of becoming an incorrect zero-calorie
			// food.
			if input.Calories == nil {
				return nil
			}

			// GetFoodByName includes logically deleted rows. Existing food data is
			// never overwritten, because foods remains the source of truth for
			// meal nutrition after the initial one-way synchronization.
			_, err = txRepository.GetFoodByName(ctx, input.Name)
			switch {
			case err == nil:
				return nil
			case !errors.Is(err, repository.ErrNotFound):
				return fmt.Errorf("check synchronized food: %w", err)
			}

			creatorID := actor.MemberID
			_, err = txRepository.CreateFood(
				ctx,
				repository.CreateFoodParams{
					Name:                input.Name,
					CaloriesPer100G:     *input.Calories,
					CarbohydratePer100G: input.Carbs,
					ProteinPer100G:      input.Protein,
					FatPer100G:          input.Fat,
					IconType:            model.FoodIconTypeBuiltin,
					IconValue:           input.IconKey,
					Source:              model.FoodSourceUser,
					CreatedBy:           &creatorID,
				},
			)

			// A concurrent request may have inserted the same food after the
			// lookup. Keeping that row is correct; this request must not overwrite
			// its nutrition data.
			if errors.Is(err, repository.ErrConflict) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("create food from material template: %w", err)
			}

			return nil
		},
	)
	if errors.Is(err, repository.ErrConflict) {
		return model.MaterialTemplate{}, ErrMaterialNameExists
	}
	if err != nil {
		return model.MaterialTemplate{}, fmt.Errorf(
			"create material template transaction: %w",
			err,
		)
	}

	return createdTemplate, nil
}

func shanghaiToday() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	n := time.Now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

func (s *PantryService) normalizeInventory(ctx context.Context, in InventoryInput) (InventoryInput, error) {
	if in.StockedOn.IsZero() {
		in.StockedOn = shanghaiToday()
	}
	loc, e := s.repository.GetStorageLocation(ctx, in.LocationID)
	if e != nil {
		return in, ErrNotFound
	}
	if !loc.Enabled {
		return in, ErrInvalidInput
	}
	if in.TemplateID != nil {
		t, e := s.repository.GetMaterialTemplate(ctx, *in.TemplateID)
		if e != nil {
			return in, ErrNotFound
		}
		if in.Name == "" {
			in.Name = t.Name
		}
		if in.IconKey == "" {
			in.IconKey = t.IconKey
		}
		if in.Unit == "" {
			in.Unit = t.DefaultUnit
		}
		if in.Calories == nil {
			in.Calories = t.CaloriesPer100G
		}
		if in.Protein == nil {
			in.Protein = t.ProteinPer100G
		}
		if in.Fat == nil {
			in.Fat = t.FatPer100G
		}
		if in.Carbs == nil {
			in.Carbs = t.CarbohydratePer100G
		}
		if in.ExpiresOn.IsZero() {
			var d *int
			if loc.StorageType == model.StorageTypeCold {
				d = t.ColdShelfLifeDays
			} else {
				d = t.AmbientShelfLifeDays
			}
			if d == nil {
				return in, fmt.Errorf("%w: template does not support storage type", ErrInvalidInput)
			}
			in.ExpiresOn = in.StockedOn.AddDate(0, 0, *d)
		}
	}
	if in.ExpiresOn.IsZero() || in.ExpiresOn.Before(in.StockedOn) || strings.TrimSpace(in.Name) == "" || in.Quantity <= 0 {
		return in, ErrInvalidInput
	}
	if in.IconKey == "" {
		in.IconKey = "generic-food"
	}
	if in.Unit == "" {
		in.Unit = "G"
	}
	return in, nil
}

func normalizeInventoryQuery(scope string, order string) (string, string) {
	if scope != "expired" {
		scope = "active"
	}

	if order != "desc" {
		order = "asc"
	}

	return scope, order
}

func normalizeInventoryQueryLimit(limit int) int {
	if limit <= 0 {
		return defaultInventoryQueryLimit
	}

	if limit > maxInventoryQueryLimit {
		return maxInventoryQueryLimit
	}

	return limit
}

func buildInventoryViews(items []model.InventoryItem) []InventoryView {
	today := shanghaiToday()
	out := make([]InventoryView, 0, len(items))

	for _, item := range items {
		out = append(out, InventoryView{
			InventoryItem: item,
			RemainingDays: int(
				item.ExpiresOn.Sub(today).Hours() / 24,
			),
			TotalDays: int(
				item.ExpiresOn.Sub(item.StockedOn).Hours() / 24,
			),
		})
	}

	return out
}

func (s *PantryService) ListInventory(ctx context.Context, scope, order string) ([]InventoryView, error) {
	scope, order = normalizeInventoryQuery(scope, order)

	items, err := s.repository.ListInventory(ctx, scope, order)
	if err != nil {
		return nil, fmt.Errorf("list inventory: %w", err)
	}

	return buildInventoryViews(items), nil
}

func (s *PantryService) SearchInventory(ctx context.Context, name string, scope string, order string, limit int) ([]InventoryView, error) {
	name = strings.TrimSpace(name)

	if name == "" || len([]rune(name)) > maxInventorySearchNameLength {
		return nil, ErrInvalidInput
	}

	scope, order = normalizeInventoryQuery(scope, order)
	limit = normalizeInventoryQueryLimit(limit)

	items, err := s.repository.SearchInventoryByName(
		ctx,
		repository.SearchInventoryByNameParams{
			Name:  name,
			Scope: scope,
			Order: order,
			Limit: limit,
		},
	)
	if errors.Is(err, repository.ErrInvalidArgument) {
		return nil, ErrInvalidInput
	}
	if err != nil {
		return nil, fmt.Errorf(
			"search inventory by name: %w",
			err,
		)
	}

	return buildInventoryViews(items), nil
}

func (s *PantryService) ListExpiringInventory(ctx context.Context, withinDays int, limit int) ([]InventoryView, error) {
	if withinDays < 0 || withinDays > maxInventoryWithinDays {
		return nil, ErrInvalidInput
	}

	limit = normalizeInventoryQueryLimit(limit)

	// 当前库存已经按照最早到期时间升序排列。
	items, err := s.ListInventory(ctx, "active", "asc")
	if err != nil {
		return nil, err
	}

	out := make([]InventoryView, 0, limit)

	for _, item := range items {
		// ListInventory的active范围不会包含负数剩余天数。
		// 因为已经按照日期升序排列，超过范围后可以直接停止。
		if item.RemainingDays > withinDays {
			break
		}

		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}

	return out, nil
}

func (s *PantryService) CreateInventory(ctx context.Context, a AuthenticatedIdentity, in InventoryInput) (InventoryView, error) {
	in, e := s.normalizeInventory(ctx, in)
	if e != nil {
		return InventoryView{}, e
	}
	v, e := s.repository.CreateInventory(ctx, repository.CreateInventoryParams{TemplateID: in.TemplateID, LocationID: in.LocationID, Name: in.Name, IconKey: in.IconKey, Quantity: in.Quantity, Unit: in.Unit, StockedOn: in.StockedOn, ExpiresOn: in.ExpiresOn, Calories: in.Calories, Protein: in.Protein, Fat: in.Fat, Carbs: in.Carbs, Description: in.Description, ImagePath: in.ImagePath, CreatedBy: a.MemberID})
	if e != nil {
		return InventoryView{}, e
	}
	return InventoryView{InventoryItem: v, RemainingDays: int(v.ExpiresOn.Sub(shanghaiToday()).Hours() / 24), TotalDays: int(v.ExpiresOn.Sub(v.StockedOn).Hours() / 24)}, nil
}

func (s *PantryService) UpdateInventory(ctx context.Context, a AuthenticatedIdentity, id, version uint64, in InventoryInput) (InventoryView, error) {
	old, e := s.repository.GetInventory(ctx, id)
	if e != nil {
		return InventoryView{}, ErrNotFound
	}
	if in.ImagePath == "" {
		in.ImagePath = old.ImagePath
	}
	in.TemplateID = old.TemplateID
	in, e = s.normalizeInventory(ctx, in)
	if e != nil {
		return InventoryView{}, e
	}
	v, e := s.repository.UpdateInventory(ctx, repository.UpdateInventoryParams{ID: id, LocationID: in.LocationID, Name: in.Name, IconKey: in.IconKey, Quantity: in.Quantity, Unit: in.Unit, StockedOn: in.StockedOn, ExpiresOn: in.ExpiresOn, Calories: in.Calories, Protein: in.Protein, Fat: in.Fat, Carbs: in.Carbs, Description: in.Description, ImagePath: in.ImagePath, Version: version})
	if errors.Is(e, repository.ErrConflict) {
		return InventoryView{}, ErrConflict
	}
	return InventoryView{InventoryItem: v, RemainingDays: int(v.ExpiresOn.Sub(shanghaiToday()).Hours() / 24), TotalDays: int(v.ExpiresOn.Sub(v.StockedOn).Hours() / 24)}, e
}

func (s *PantryService) Discard(ctx context.Context, a AuthenticatedIdentity, id, version uint64, reason string) error {
	e := s.repository.DiscardInventory(ctx, id, a.MemberID, version, strings.TrimSpace(reason))
	if errors.Is(e, repository.ErrConflict) {
		return ErrConflict
	}
	return e
}

func milestoneFor(total, remaining int) (model.ReminderMilestone, bool) {
	if total <= 1 || remaining < 0 {
		return "", false
	}
	if remaining <= 1 {
		return model.ReminderLastDay, true
	}
	half := (total + 1) / 2
	if total >= 7 {
		quarter := (total + 3) / 4
		if remaining <= quarter {
			return model.ReminderThreeQuarters, true
		}
	}
	if remaining <= half {
		return model.ReminderHalf, true
	}
	return "", false
}

func (s *PantryService) Notifications(ctx context.Context, a AuthenticatedIdentity) (NotificationSummary, error) {
	var out NotificationSummary
	if a.Role == model.MemberRoleOwner {
		n, e := s.repository.CountMembersByStatus(ctx, a.HouseholdID, model.MemberStatusPending)
		if e != nil {
			return out, e
		}
		out.PendingMembers = n
	}
	items, e := s.ListInventory(ctx, "active", "asc")
	if e != nil {
		return out, e
	}
	out.MaterialReminders = make([]MaterialReminder, 0)
	for _, v := range items {
		m, ok := milestoneFor(v.TotalDays, v.RemainingDays)
		if !ok {
			continue
		}
		read, e := s.repository.ReminderRead(ctx, a.MemberID, v.ID, m)
		if e != nil {
			return out, e
		}
		if read {
			continue
		}
		label := map[model.ReminderMilestone]string{model.ReminderHalf: "已用过半", model.ReminderThreeQuarters: "已用过75%", model.ReminderLastDay: "仅剩最后一天"}[m]
		out.MaterialReminders = append(out.MaterialReminders, MaterialReminder{ItemID: v.ID, ItemName: v.Name, Milestone: m, RemainingDays: v.RemainingDays, Message: label})
	}
	return out, nil
}

func (s *PantryService) ReadReminder(ctx context.Context, a AuthenticatedIdentity, itemID uint64, m model.ReminderMilestone) error {
	if m != model.ReminderHalf && m != model.ReminderThreeQuarters && m != model.ReminderLastDay {
		return ErrInvalidInput
	}
	return s.repository.MarkReminderRead(ctx, a.MemberID, itemID, m)
}
