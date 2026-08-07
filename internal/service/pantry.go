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

type PantryService struct {
	repository *repository.Repository
	weather    *weatherClient
}

func NewPantryService(r *repository.Repository) *PantryService {
	return &PantryService{repository: r, weather: newWeatherClient()}
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
func (s *PantryService) CreateTemplate(ctx context.Context, a AuthenticatedIdentity, in CreateTemplateInput) (model.MaterialTemplate, error) {
	if !canManagePantry(a) {
		return model.MaterialTemplate{}, ErrForbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || (in.ColdDays == nil && in.AmbientDays == nil) {
		return model.MaterialTemplate{}, ErrInvalidInput
	}
	if in.IconKey == "" {
		in.IconKey = "generic-food"
	}
	if in.DefaultUnit == "" {
		in.DefaultUnit = "G"
	}
	v, e := s.repository.CreateMaterialTemplate(ctx, repository.CreateTemplateParams{Name: in.Name, IconKey: in.IconKey, DefaultUnit: in.DefaultUnit, ColdDays: in.ColdDays, AmbientDays: in.AmbientDays, Calories: in.Calories, Protein: in.Protein, Fat: in.Fat, Carbs: in.Carbs})
	if errors.Is(e, repository.ErrConflict) {
		return v, ErrMaterialNameExists
	}
	return v, e
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
func (s *PantryService) ListInventory(ctx context.Context, scope, order string) ([]InventoryView, error) {
	if scope != "expired" {
		scope = "active"
	}
	if order != "desc" {
		order = "asc"
	}
	items, e := s.repository.ListInventory(ctx, scope, order)
	if e != nil {
		return nil, e
	}
	today := shanghaiToday()
	out := make([]InventoryView, 0, len(items))
	for _, v := range items {
		out = append(out, InventoryView{InventoryItem: v, RemainingDays: int(v.ExpiresOn.Sub(today).Hours() / 24), TotalDays: int(v.ExpiresOn.Sub(v.StockedOn).Hours() / 24)})
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
