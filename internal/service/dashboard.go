package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"vhome/internal/model"
	"vhome/internal/repository"
)

// DashboardService is an application-level read service. It composes pantry,
// expense, member and weather data without making any one domain service own
// the dashboard itself.
type DashboardService struct {
	repository *repository.Repository
	pantry     *PantryService
	expense    *ExpenseService
	weather    *weatherClient
}

func NewDashboardService(repo *repository.Repository, pantry *PantryService, expense *ExpenseService) *DashboardService {
	return &DashboardService{
		repository: repo,
		pantry:     pantry,
		expense:    expense,
		weather:    newWeatherClient(),
	}
}

type DashboardStorage struct {
	StorageType   model.StorageType `json:"storage_type"`
	LocationCount uint64            `json:"location_count"`
	ItemCount     uint64            `json:"item_count"`
	Percentage    int               `json:"percentage"`
}

type DashboardActivity struct {
	Type       string    `json:"type"`
	Text       string    `json:"text"`
	HappenedAt time.Time `json:"happened_at"`
}

type DashboardMember struct {
	ID             uint64               `json:"id"`
	DisplayName    string               `json:"display_name"`
	Role           model.MemberRole     `json:"role"`
	PresenceStatus model.PresenceStatus `json:"presence_status"`
	AvatarKey      model.MemberAvatar   `json:"avatar_key"`
	Version        uint64               `json:"version"`
}

type WeatherSummary struct {
	Available       bool    `json:"available"`
	City            string  `json:"city"`
	TemperatureMean float64 `json:"temperature_mean"`
	WeatherCode     int     `json:"weather_code"`
	Description     string  `json:"description"`
	Icon            string  `json:"icon"`
	Date            string  `json:"date"`
}

type DashboardSummary struct {
	HouseholdName  string                  `json:"household_name"`
	Province       string                  `json:"province"`
	City           string                  `json:"city"`
	InventoryCount int                     `json:"inventory_count"`
	AttentionCount int                     `json:"attention_count"`
	DueTodayCount  int                     `json:"due_today_count"`
	PantryWatch    []InventoryView         `json:"pantry_watch"`
	Storage        []DashboardStorage      `json:"storage"`
	Activities     []DashboardActivity     `json:"activities"`
	Members        []DashboardMember       `json:"members"`
	Weather        WeatherSummary          `json:"weather"`
	MonthlyExpense HouseholdExpenseSummary `json:"monthly_expense"`
}

func (s *DashboardService) Dashboard(ctx context.Context, actor AuthenticatedIdentity) (DashboardSummary, error) {
	household, err := s.repository.GetHouseholdByID(ctx, actor.HouseholdID)
	if err != nil {
		return DashboardSummary{}, err
	}
	items, err := s.pantry.ListInventory(ctx, "active", "asc")
	if err != nil {
		return DashboardSummary{}, err
	}
	members, err := s.repository.ListMembers(ctx, actor.HouseholdID)
	if err != nil {
		return DashboardSummary{}, err
	}
	storageStats, err := s.repository.DashboardStorageStats(ctx)
	if err != nil {
		return DashboardSummary{}, err
	}
	activityRows, err := s.repository.DashboardActivities(ctx, 6)
	if err != nil {
		return DashboardSummary{}, err
	}
	monthlyExpense, err := s.expense.CurrentHouseholdSummary(ctx, actor)
	if err != nil {
		return DashboardSummary{}, err
	}

	out := DashboardSummary{
		HouseholdName:  household.DisplayName,
		Province:       household.Province,
		City:           household.City,
		InventoryCount: len(items),
		PantryWatch:    make([]InventoryView, 0),
		Storage:        make([]DashboardStorage, 0, 2),
		Activities:     make([]DashboardActivity, 0, len(activityRows)),
		Members:        make([]DashboardMember, 0, len(members)),
		MonthlyExpense: monthlyExpense,
	}

	for _, item := range items {
		if item.RemainingDays >= 0 && item.RemainingDays <= 7 {
			out.AttentionCount++
			if len(out.PantryWatch) < 5 {
				out.PantryWatch = append(out.PantryWatch, item)
			}
		}
		if item.RemainingDays == 0 {
			out.DueTodayCount++
		}
	}

	for _, stat := range storageStats {
		percentage := 0
		if len(items) > 0 {
			percentage = int(math.Round(float64(stat.ItemCount) / float64(len(items)) * 100))
		}
		out.Storage = append(out.Storage, DashboardStorage{
			StorageType:   stat.StorageType,
			LocationCount: stat.LocationCount,
			ItemCount:     stat.ItemCount,
			Percentage:    percentage,
		})
	}

	for _, row := range activityRows {
		text := ""
		switch row.Type {
		case "INVENTORY_ADDED":
			text = fmt.Sprintf("%s 添加了 %s", row.ActorName, row.ItemName)
		case "INVENTORY_DISCARDED":
			text = fmt.Sprintf("%s 丢弃了 %s", row.ActorName, row.ItemName)
		case "MEMBER_JOINED":
			text = fmt.Sprintf("%s 加入了家庭", row.ActorName)
		}
		out.Activities = append(out.Activities, DashboardActivity{
			Type: row.Type, Text: text, HappenedAt: row.HappenedAt,
		})
	}

	for _, member := range members {
		if member.Status != model.MemberStatusActive {
			continue
		}
		out.Members = append(out.Members, DashboardMember{
			ID:             member.ID,
			DisplayName:    member.DisplayName,
			Role:           member.Role,
			PresenceStatus: member.PresenceStatus,
			AvatarKey:      member.AvatarKey,
			Version:        member.Version,
		})
	}

	out.Weather = s.weather.get(ctx, household.Province, household.City)
	return out, nil
}

type weatherCacheEntry struct {
	value     WeatherSummary
	expiresAt time.Time
}

type weatherClient struct {
	httpClient *http.Client
	mu         sync.Mutex
	cache      map[string]weatherCacheEntry
}

func newWeatherClient() *weatherClient {
	return &weatherClient{
		httpClient: &http.Client{Timeout: 4 * time.Second},
		cache:      make(map[string]weatherCacheEntry),
	}
}

func (w *weatherClient) get(ctx context.Context, province, city string) WeatherSummary {
	province = strings.TrimSpace(province)
	city = strings.TrimSpace(city)
	if city == "" {
		return WeatherSummary{City: city}
	}

	now := time.Now()
	key := province + "/" + city + "/" + now.Format("2006-01-02")
	w.mu.Lock()
	entry, ok := w.cache[key]
	w.mu.Unlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.value
	}

	value, err := w.fetch(ctx, province, city)
	if err != nil {
		value = WeatherSummary{City: city}
	}
	expiresAt := now.Add(10 * time.Minute)
	if value.Available {
		expiresAt = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, now.Location())
	}
	w.mu.Lock()
	w.cache[key] = weatherCacheEntry{value: value, expiresAt: expiresAt}
	w.mu.Unlock()
	return value
}

func (w *weatherClient) fetch(ctx context.Context, province, city string) (WeatherSummary, error) {
	cityCode, err := w.findChinaWeatherCityCode(ctx, province, city)
	if err != nil {
		return WeatherSummary{}, err
	}

	endpoint := "https://d1.weather.com.cn/weather_index/" + cityCode + ".html"
	payload, err := w.getText(ctx, endpoint, "https://www.weather.com.cn/")
	if err != nil {
		return WeatherSummary{}, err
	}

	var current struct {
		WeatherInfo struct {
			City    string `json:"city"`
			Weather string `json:"weather"`
			FCTime  string `json:"fctime"`
		} `json:"weatherinfo"`
	}
	if err := json.Unmarshal(extractJavaScriptValue(payload, "var cityDZ ="), &current); err != nil {
		return WeatherSummary{}, fmt.Errorf("decode China Weather current forecast: %w", err)
	}

	var forecast struct {
		Days []struct {
			High string `json:"fc"`
			Low  string `json:"fd"`
		} `json:"f"`
	}
	if err := json.Unmarshal(extractJavaScriptValue(payload, "var fc ="), &forecast); err != nil {
		return WeatherSummary{}, fmt.Errorf("decode China Weather daily forecast: %w", err)
	}
	if len(forecast.Days) == 0 {
		return WeatherSummary{}, fmt.Errorf("China Weather daily forecast is empty")
	}
	high, err := strconv.ParseFloat(forecast.Days[0].High, 64)
	if err != nil {
		return WeatherSummary{}, fmt.Errorf("parse Celsius high temperature: %w", err)
	}
	low, err := strconv.ParseFloat(forecast.Days[0].Low, 64)
	if err != nil {
		return WeatherSummary{}, fmt.Errorf("parse Celsius low temperature: %w", err)
	}

	description := strings.TrimSpace(current.WeatherInfo.Weather)
	code, icon := describeChineseWeather(description)
	date := time.Now().Format("2006-01-02")
	if len(current.WeatherInfo.FCTime) >= 8 {
		if parsed, parseErr := time.Parse("20060102", current.WeatherInfo.FCTime[:8]); parseErr == nil {
			date = parsed.Format("2006-01-02")
		}
	}
	return WeatherSummary{
		Available:       true,
		City:            city,
		TemperatureMean: math.Round((high+low)/2*10) / 10,
		WeatherCode:     code,
		Description:     description,
		Icon:            icon,
		Date:            date,
	}, nil
}

func (w *weatherClient) findChinaWeatherCityCode(ctx context.Context, province, city string) (string, error) {
	endpoint, _ := url.Parse("https://toy1.weather.com.cn/search")
	query := endpoint.Query()
	query.Set("cityname", normalizeChineseLocation(city))
	endpoint.RawQuery = query.Encode()

	payload, err := w.getText(ctx, endpoint.String(), "https://www.weather.com.cn/")
	if err != nil {
		return "", err
	}
	searchJSON := strings.TrimSpace(string(payload))
	searchJSON = strings.TrimPrefix(searchJSON, "(")
	searchJSON = strings.TrimSuffix(searchJSON, ")")
	var results []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal([]byte(searchJSON), &results); err != nil {
		return "", fmt.Errorf("decode China Weather city search: %w", err)
	}

	cityKey := normalizeChineseLocation(city)
	provinceKey := normalizeChineseLocation(province)
	fallback := ""
	for _, result := range results {
		parts := strings.Split(result.Ref, "~")
		if len(parts) < 10 || normalizeChineseLocation(parts[2]) != cityKey || !isWeatherCityCode(parts[0]) {
			continue
		}
		if fallback == "" {
			fallback = parts[0]
		}
		if provinceKey == "" || normalizeChineseLocation(parts[len(parts)-1]) == provinceKey {
			return parts[0], nil
		}
	}
	if fallback != "" && provinceKey == "" {
		return fallback, nil
	}
	return "", fmt.Errorf("China Weather city not found for %s/%s", province, city)
}

func isWeatherCityCode(value string) bool {
	if len(value) != 9 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func normalizeChineseLocation(value string) string {
	value = strings.TrimSpace(value)
	for _, suffix := range []string{
		"维吾尔自治区", "壮族自治区", "回族自治区", "特别行政区",
		"自治区", "自治州", "地区", "省", "市", "盟", "县", "区",
	} {
		if strings.HasSuffix(value, suffix) {
			return strings.TrimSuffix(value, suffix)
		}
	}
	return value
}

func (w *weatherClient) getText(ctx context.Context, endpoint, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VHome/0.1; +https://weather.com.cn/)")
	req.Header.Set("Referer", referer)
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned %s", resp.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read weather API response: %w", err)
	}
	return payload, nil
}

func extractJavaScriptValue(payload []byte, marker string) []byte {
	text := string(payload)
	start := strings.Index(text, marker)
	if start < 0 {
		return nil
	}
	value := text[start+len(marker):]
	if end := strings.Index(value, ";var "); end >= 0 {
		value = value[:end]
	}
	return []byte(strings.TrimSpace(strings.TrimSuffix(value, ";")))
}

func describeChineseWeather(description string) (int, string) {
	switch {
	case strings.Contains(description, "雷"):
		return 95, "⛈️"
	case strings.Contains(description, "雪"):
		return 71, "🌨️"
	case strings.Contains(description, "雨"):
		return 61, "🌧️"
	case strings.Contains(description, "雾") || strings.Contains(description, "霾"):
		return 45, "🌫️"
	case strings.Contains(description, "阴"):
		return 3, "☁️"
	case strings.Contains(description, "多云"):
		return 2, "⛅"
	case strings.Contains(description, "晴"):
		return 0, "☀️"
	default:
		return -1, "🌤️"
	}
}
