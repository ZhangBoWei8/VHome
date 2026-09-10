package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vhome/internal/model"
	"vhome/internal/repository"
)

var ErrCalendarNoticeUnavailable = errors.New("service: State Council holiday notice is not available")

type CalendarSyncService struct {
	repository *repository.Repository
	httpClient *http.Client
	location   *time.Location
}

func NewCalendarSyncService(repo *repository.Repository) (*CalendarSyncService, error) {
	if repo == nil {
		return nil, errors.New("calendar sync repository is nil")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load calendar sync location: %w", err)
	}
	return &CalendarSyncService{
		repository: repo,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		location:   location,
	}, nil
}

func (s *CalendarSyncService) SyncYear(ctx context.Context, actor AuthenticatedIdentity, year int) (uint64, error) {
	if err := requireOwner(actor); err != nil {
		return 0, err
	}
	return s.syncYear(ctx, year, true)
}

// SyncUpcoming is used by the background worker. Before October the next
// year's notice normally does not exist, so it only checks the current year.
func (s *CalendarSyncService) SyncUpcoming(ctx context.Context, now time.Time) (uint64, error) {
	now = now.In(s.location)
	targetYear := now.Year()
	if now.Month() >= time.October {
		targetYear++
	}
	return s.syncYear(ctx, targetYear, false)
}

func (s *CalendarSyncService) syncYear(ctx context.Context, year int, force bool) (uint64, error) {
	if year < 2024 || year > time.Now().In(s.location).Year()+2 {
		return 0, ErrInvalidInput
	}
	if !force {
		count, err := s.repository.CountCalendarDayOverridesByYear(ctx, year)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return 0, nil
		}
	}
	noticeURL, err := s.findOfficialNotice(ctx, year)
	if err != nil {
		return 0, err
	}
	payload, err := s.get(ctx, noticeURL)
	if err != nil {
		return 0, err
	}
	overrides, err := parseStateCouncilHolidayNotice(payload, noticeURL, year, s.location)
	if err != nil {
		return 0, err
	}
	err = s.repository.WithinTransaction(ctx, func(tx *repository.Repository) error {
		for _, item := range overrides {
			if err := tx.UpsertCalendarDayOverride(ctx, item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("save official calendar overrides: %w", err)
	}
	return uint64(len(overrides)), nil
}

func (s *CalendarSyncService) findOfficialNotice(ctx context.Context, year int) (string, error) {
	if year == 2026 {
		return "https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm", nil
	}
	title := fmt.Sprintf("国务院办公厅关于%d年部分节假日安排的通知", year)
	endpoint := "https://sousuo.www.gov.cn/zcwjk/policyDocumentLibrary?q=" + url.QueryEscape(title) + "&t=zhengcelibrary"
	payload, err := s.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	text := string(payload)
	linkPattern := regexp.MustCompile(`(?i)(https?://(?:www\.)?gov\.cn)?(/zhengce/[^"'<> ]*content_[0-9]+\.htm)`)
	for _, match := range linkPattern.FindAllStringSubmatch(text, -1) {
		candidate := match[1] + match[2]
		if match[1] == "" {
			candidate = "https://www.gov.cn" + match[2]
		}
		page, fetchErr := s.get(ctx, candidate)
		if fetchErr == nil && strings.Contains(stripHTML(string(page)), title) {
			return candidate, nil
		}
	}
	return "", ErrCalendarNoticeUnavailable
}

func (s *CalendarSyncService) get(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create official calendar request: %w", err)
	}
	request.Header.Set("User-Agent", "VHome/1.0 (+family calendar sync)")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download official calendar page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download official calendar page: HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read official calendar page: %w", err)
	}
	return payload, nil
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripHTML(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), "")
}

func parseStateCouncilHolidayNotice(payload []byte, sourceURL string, year int, location *time.Location) ([]model.CalendarDayOverride, error) {
	text := stripHTML(string(payload))
	if !strings.Contains(text, strconv.Itoa(year)+"年部分节假日安排") {
		return nil, errors.New("official calendar notice year does not match")
	}
	holidayNames := []string{"元旦", "春节", "清明节", "劳动节", "端午节", "中秋节", "国庆节"}
	result := make(map[string]model.CalendarDayOverride)
	now := time.Now().In(location)
	for index, name := range holidayNames {
		startIndex := strings.Index(text, name+"：")
		if startIndex < 0 {
			return nil, fmt.Errorf("official calendar notice is missing %s", name)
		}
		segmentEnd := len(text)
		if index+1 < len(holidayNames) {
			nextStart := startIndex + len(name) + len("：")
			if next := strings.Index(text[nextStart:], holidayNames[index+1]+"："); next >= 0 {
				segmentEnd = nextStart + next
			}
		}
		segment := text[startIndex:segmentEnd]
		periodEnd := strings.Index(segment, "放假")
		if periodEnd < 0 {
			return nil, fmt.Errorf("official calendar notice has no holiday period for %s", name)
		}
		period := segment[:periodEnd]
		startMonth, startDay, endMonth, endDay, err := parseHolidayRange(period)
		if err != nil {
			return nil, fmt.Errorf("parse %s holiday range: %w", name, err)
		}
		start := time.Date(year, time.Month(startMonth), startDay, 0, 0, 0, 0, location)
		end := time.Date(year, time.Month(endMonth), endDay, 0, 0, 0, 0, location)
		if end.Before(start) || end.Sub(start) > 15*24*time.Hour {
			return nil, fmt.Errorf("official calendar notice has invalid %s range", name)
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			result[day.Format("2006-01-02")] = model.CalendarDayOverride{
				CalendarDate: day, DayType: model.CalendarDayHoliday, HolidayName: name,
				SourceURL: sourceURL, SyncedAt: now,
			}
		}
		if workIndex := strings.Index(segment, "上班"); workIndex >= 0 {
			workPart := segment[periodEnd:workIndex]
			for _, date := range parseFullMonthDays(workPart, year, location) {
				result[date.Format("2006-01-02")] = model.CalendarDayOverride{
					CalendarDate: date, DayType: model.CalendarDayTransferWorkday,
					SourceURL: sourceURL, SyncedAt: now,
				}
			}
		}
	}
	if len(result) < 20 {
		return nil, errors.New("official calendar notice produced too few override days")
	}
	overrides := make([]model.CalendarDayOverride, 0, len(result))
	for _, item := range result {
		overrides = append(overrides, item)
	}
	sort.Slice(overrides, func(i, j int) bool {
		return overrides[i].CalendarDate.Before(overrides[j].CalendarDate)
	})
	return overrides, nil
}

var holidayRangePattern = regexp.MustCompile(`([0-9]{1,2})月([0-9]{1,2})日.*?至(?:([0-9]{1,2})月)?([0-9]{1,2})日`)
var singleHolidayPattern = regexp.MustCompile(`([0-9]{1,2})月([0-9]{1,2})日`)
var fullMonthDayPattern = regexp.MustCompile(`([0-9]{1,2})月([0-9]{1,2})日`)

func parseHolidayRange(value string) (int, int, int, int, error) {
	match := holidayRangePattern.FindStringSubmatch(value)
	if len(match) == 5 {
		startMonth, _ := strconv.Atoi(match[1])
		startDay, _ := strconv.Atoi(match[2])
		endMonth := startMonth
		if match[3] != "" {
			endMonth, _ = strconv.Atoi(match[3])
		}
		endDay, _ := strconv.Atoi(match[4])
		return startMonth, startDay, endMonth, endDay, nil
	}
	match = singleHolidayPattern.FindStringSubmatch(value)
	if len(match) == 3 {
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		return month, day, month, day, nil
	}
	return 0, 0, 0, 0, errors.New("holiday date not found")
}

func parseFullMonthDays(value string, year int, location *time.Location) []time.Time {
	matches := fullMonthDayPattern.FindAllStringSubmatch(value, -1)
	result := make([]time.Time, 0, len(matches))
	for _, match := range matches {
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		result = append(result, time.Date(year, time.Month(month), day, 0, 0, 0, 0, location))
	}
	return result
}
