package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/service"
)

type MemoHandler struct {
	service      *service.MemoService
	calendarSync *service.CalendarSyncService
	location     *time.Location
}

func NewMemoHandler(memoService *service.MemoService, calendarSync *service.CalendarSyncService) *MemoHandler {
	location, _ := time.LoadLocation("Asia/Shanghai")
	return &MemoHandler{service: memoService, calendarSync: calendarSync, location: location}
}

type memoRequest struct {
	Title        string   `json:"title" binding:"required,max=128"`
	Description  string   `json:"description"`
	RemindAt     string   `json:"remind_at" binding:"required"`
	RecipientIDs []uint64 `json:"recipient_ids" binding:"required"`
	Version      uint64   `json:"version"`
}

type memoVersionRequest struct {
	Version uint64 `json:"version" binding:"required"`
}

func (h *MemoHandler) MemberOptions(c *gin.Context) {
	data, err := h.service.MemberOptions(c.Request.Context(), currentActor(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Mine(c *gin.Context) {
	limit, offset, err := parseMemoPagination(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.MyMemos(c.Request.Context(), currentActor(c), limit, offset)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Calendar(c *gin.Context) {
	data, err := h.service.MyMemoCalendar(c.Request.Context(), currentActor(c), c.Query("month"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Day(c *gin.Context) {
	data, err := h.service.MyMemosForDay(c.Request.Context(), currentActor(c), c.Query("date"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Search(c *gin.Context) {
	limit, err := parseMemoIntQuery(c, "limit", 100)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.SearchMyMemos(c.Request.Context(), currentActor(c), c.Query("keyword"), limit)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) CreatedByMe(c *gin.Context) {
	limit, offset, err := parseMemoPagination(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.CreatedMemos(c.Request.Context(), currentActor(c), limit, offset)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Get(c *gin.Context) {
	memoID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.Memo(c.Request.Context(), currentActor(c), memoID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Create(c *gin.Context) {
	request, remindAt, ok := h.bindMemoRequest(c, false)
	if !ok {
		return
	}
	data, err := h.service.CreateMemo(c.Request.Context(), currentActor(c), service.CreateMemoInput{
		Title: request.Title, Description: request.Description,
		RemindAt: remindAt, RecipientIDs: request.RecipientIDs,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusCreated, data)
}

func (h *MemoHandler) Update(c *gin.Context) {
	memoID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	request, remindAt, ok := h.bindMemoRequest(c, true)
	if !ok {
		return
	}
	data, err := h.service.UpdateMemo(c.Request.Context(), currentActor(c), memoID, service.UpdateMemoInput{
		Title: request.Title, Description: request.Description, RemindAt: remindAt,
		RecipientIDs: request.RecipientIDs, Version: request.Version,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *MemoHandler) Delete(c *gin.Context) {
	memoID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request memoVersionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.service.DeleteMemo(c.Request.Context(), currentActor(c), memoID, request.Version); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MemoHandler) Dismiss(c *gin.Context) {
	memoID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request memoVersionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.DismissMemo(c.Request.Context(), currentActor(c), memoID, request.Version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

type calendarSyncRequest struct {
	Year int `json:"year" binding:"required"`
}

func (h *MemoHandler) SyncCalendar(c *gin.Context) {
	var request calendarSyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	count, err := h.calendarSync.SyncYear(c.Request.Context(), currentActor(c), request.Year)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, gin.H{"synced_days": count})
}

func (h *MemoHandler) bindMemoRequest(c *gin.Context, requireVersion bool) (memoRequest, time.Time, bool) {
	var request memoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return memoRequest{}, time.Time{}, false
	}
	if requireVersion && request.Version == 0 {
		writeBadRequest(c, fmt.Errorf("invalid version"))
		return memoRequest{}, time.Time{}, false
	}
	remindAt, err := parseMemoTime(request.RemindAt, h.location)
	if err != nil {
		writeBadRequest(c, err)
		return memoRequest{}, time.Time{}, false
	}
	return request, remindAt, true
}

func parseMemoTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.In(location), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid remind_at")
	}
	return parsed, nil
}

func parseMemoPagination(c *gin.Context) (int, int, error) {
	limit, err := parseMemoIntQuery(c, "limit", 100)
	if err != nil {
		return 0, 0, err
	}
	offset, err := parseMemoIntQuery(c, "offset", 0)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}

func parseMemoIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
}
