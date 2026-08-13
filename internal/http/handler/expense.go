package handler

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

type ExpenseHandler struct {
	service  *service.ExpenseService
	location *time.Location
}

func NewExpenseHandler(expenseService *service.ExpenseService) *ExpenseHandler {
	location, _ := time.LoadLocation("Asia/Shanghai")
	return &ExpenseHandler{service: expenseService, location: location}
}

type expenseRequest struct {
	CategoryID   uint16             `json:"category_id" binding:"required"`
	ExpenseScope model.ExpenseScope `json:"expense_scope" binding:"required"`
	Title        string             `json:"title" binding:"required,max=128"`
	Amount       string             `json:"amount" binding:"required"`
	SpentOn      string             `json:"spent_on" binding:"required"`
	Note         string             `json:"note" binding:"max=500"`
	Version      uint64             `json:"version"`
}

func (h *ExpenseHandler) Categories(c *gin.Context) {
	categories, err := h.service.Categories(c.Request.Context(), currentActor(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, categories)
}

func (h *ExpenseHandler) MyMonthly(c *gin.Context) {
	data, err := h.service.MyMonthlyExpenses(c.Request.Context(), currentActor(c), c.Query("month"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *ExpenseHandler) CollectiveMonthly(c *gin.Context) {
	data, err := h.service.CollectiveMonthlyExpenses(c.Request.Context(), currentActor(c), c.Query("month"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *ExpenseHandler) Create(c *gin.Context) {
	input, ok := h.bindInput(c, false)
	if !ok {
		return
	}
	created, err := h.service.CreateExpense(c.Request.Context(), currentActor(c), input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusCreated, created)
}

func (h *ExpenseHandler) Update(c *gin.Context) {
	expenseID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	input, ok := h.bindInput(c, true)
	if !ok {
		return
	}
	updated, err := h.service.UpdateExpense(c.Request.Context(), currentActor(c), expenseID, input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, updated)
}

type deleteExpenseRequest struct {
	Version uint64 `json:"version" binding:"required"`
}

func (h *ExpenseHandler) Delete(c *gin.Context) {
	expenseID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request deleteExpenseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.service.DeleteExpense(c.Request.Context(), currentActor(c), expenseID, request.Version); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ExpenseHandler) bindInput(c *gin.Context, requireVersion bool) (service.ExpenseInput, bool) {
	var request expenseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return service.ExpenseInput{}, false
	}
	amountCents, err := parseAmountCents(request.Amount)
	if err != nil {
		writeBadRequest(c, err)
		return service.ExpenseInput{}, false
	}
	spentOn, err := time.ParseInLocation("2006-01-02", request.SpentOn, h.location)
	if err != nil {
		writeBadRequest(c, fmt.Errorf("invalid spent_on: %w", err))
		return service.ExpenseInput{}, false
	}
	if requireVersion && request.Version == 0 {
		writeBadRequest(c, fmt.Errorf("invalid version"))
		return service.ExpenseInput{}, false
	}
	return service.ExpenseInput{
		CategoryID:   request.CategoryID,
		ExpenseScope: request.ExpenseScope,
		Title:        request.Title,
		AmountCents:  amountCents,
		SpentOn:      spentOn,
		Note:         request.Note,
		Version:      request.Version,
	}, true
}

// parseAmountCents converts a decimal string to integer cents without passing
// through float64, so values such as 19.90 cannot lose a cent to rounding.
func parseAmountCents(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("invalid amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && len(parts[1]) > 2) {
		return 0, fmt.Errorf("invalid amount")
	}
	whole, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || whole > math.MaxUint64/100 {
		return 0, fmt.Errorf("invalid amount")
	}
	fraction := uint64(0)
	if len(parts) == 2 && parts[1] != "" {
		fractionText := parts[1]
		if len(fractionText) == 1 {
			fractionText += "0"
		}
		fraction, err = strconv.ParseUint(fractionText, 10, 8)
		if err != nil {
			return 0, fmt.Errorf("invalid amount")
		}
	}
	if whole == math.MaxUint64/100 && fraction > math.MaxUint64-whole*100 {
		return 0, fmt.Errorf("invalid amount")
	}
	cents := whole*100 + fraction
	if cents == 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	return cents, nil
}

func (h *ExpenseHandler) Export(c *gin.Context) {
	start, err := time.ParseInLocation("2006-01-02", c.Query("from"), h.location)
	if err != nil {
		writeBadRequest(c, fmt.Errorf("invalid from: %w", err))
		return
	}
	end, err := time.ParseInLocation("2006-01-02", c.Query("to"), h.location)
	if err != nil {
		writeBadRequest(c, fmt.Errorf("invalid to: %w", err))
		return
	}
	view := service.ExpenseExportView(strings.ToUpper(strings.TrimSpace(c.Query("view"))))
	records, err := h.service.ExportExpenses(c.Request.Context(), currentActor(c), start, end.AddDate(0, 0, 1), view)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	filename := fmt.Sprintf("vhome-expenses-%s-%s.csv", start.Format("20060102"), end.Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"日期", "支出名称", "分类", "类型", "金额（元）", "记录成员", "备注"})
	for _, record := range records {
		_ = writer.Write([]string{
			record.SpentOn.Format("2006-01-02"),
			csvSafe(record.Title),
			csvSafe(record.Category.Name),
			expenseScopeText(record.ExpenseScope),
			formatCents(record.AmountCents),
			csvSafe(record.MemberName),
			csvSafe(record.Note),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = c.Error(err)
	}
}

func csvSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@", rune(value[0])) {
		return "'" + value
	}
	return value
}

func expenseScopeText(scope model.ExpenseScope) string {
	if scope == model.ExpenseScopeCollective {
		return "集体开销"
	}
	return "个人开销"
}

func formatCents(cents uint64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}
