package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/middleware"
	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

const (
	maxFoodImageBytes = int64(5 << 20)
	maxFoodFormBytes  = int64(8 << 20)
)

type MealHandler struct {
	service   *service.MealService
	uploadDir string
}

func NewMealHandler(mealService *service.MealService, uploadDir string) *MealHandler {
	return &MealHandler{service: mealService, uploadDir: uploadDir}
}

func currentActor(c *gin.Context) service.AuthenticatedIdentity {
	actor, _ := middleware.CurrentIdentity(c)
	return actor
}

func parseID(c *gin.Context, name string) (uint64, error) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func parseRequiredFloat(value, field string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

// parseOptionalFloat distinguishes an omitted nutrient from zero. This is
// important because zero is valid data, while nil means the daily macro total
// is incomplete and must display a warning.
func parseOptionalFloat(value, field string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s", field)
	}
	return &parsed, nil
}

func parseOptionalBool(value, field string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

type foodFormValues struct {
	name                     string
	caloriesPer100G          float64
	carbohydratePer100G      *float64
	proteinPer100G           *float64
	fatPer100G               *float64
	iconType                 model.FoodIconType
	iconValue                string
	confirmNutritionMismatch bool
	version                  uint64
}

func parseFoodForm(c *gin.Context, requireVersion bool) (foodFormValues, error) {
	calories, err := parseRequiredFloat(c.PostForm("calories_per_100g"), "calories_per_100g")
	if err != nil {
		return foodFormValues{}, err
	}
	carbohydrate, err := parseOptionalFloat(c.PostForm("carbohydrate_per_100g"), "carbohydrate_per_100g")
	if err != nil {
		return foodFormValues{}, err
	}
	protein, err := parseOptionalFloat(c.PostForm("protein_per_100g"), "protein_per_100g")
	if err != nil {
		return foodFormValues{}, err
	}
	fat, err := parseOptionalFloat(c.PostForm("fat_per_100g"), "fat_per_100g")
	if err != nil {
		return foodFormValues{}, err
	}
	confirmed, err := parseOptionalBool(c.PostForm("confirm_nutrition_mismatch"), "confirm_nutrition_mismatch")
	if err != nil {
		return foodFormValues{}, err
	}

	values := foodFormValues{
		name:                     c.PostForm("name"),
		caloriesPer100G:          calories,
		carbohydratePer100G:      carbohydrate,
		proteinPer100G:           protein,
		fatPer100G:               fat,
		iconType:                 model.FoodIconType(strings.ToUpper(strings.TrimSpace(c.PostForm("icon_type")))),
		iconValue:                strings.TrimSpace(c.PostForm("icon_value")),
		confirmNutritionMismatch: confirmed,
	}
	if requireVersion {
		values.version, err = strconv.ParseUint(c.PostForm("version"), 10, 64)
		if err != nil || values.version == 0 {
			return foodFormValues{}, fmt.Errorf("invalid version")
		}
	}
	return values, nil
}

// saveFoodImage validates file content rather than trusting the extension,
// assigns an unpredictable server-side name, and returns a relative path that
// is safe to expose below /uploads.
func (h *MealHandler) saveFoodImage(c *gin.Context) (string, error) {
	fileHeader, err := c.FormFile("image")
	if errors.Is(err, http.ErrMissingFile) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxFoodImageBytes {
		return "", fmt.Errorf("image must not exceed 5 MiB")
	}

	source, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer source.Close()

	header := make([]byte, 512)
	read, err := source.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	extension, ok := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}[http.DetectContentType(header[:read])]
	if !ok {
		return "", fmt.Errorf("image must be JPEG, PNG, or WebP")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate image name: %w", err)
	}
	relativePath := filepath.Join("foods", time.Now().Format("2006/01"), hex.EncodeToString(randomBytes)+extension)
	absolutePath := filepath.Join(h.uploadDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return "", fmt.Errorf("create image directory: %w", err)
	}

	destination, err := os.OpenFile(absolutePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create image: %w", err)
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxFoodImageBytes+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil || written > maxFoodImageBytes {
		_ = os.Remove(absolutePath)
		if copyErr != nil {
			return "", fmt.Errorf("save image: %w", copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close image: %w", closeErr)
		}
		return "", fmt.Errorf("image must not exceed 5 MiB")
	}
	return filepath.ToSlash(relativePath), nil
}

// removeFoodImage only accepts paths below the server-generated foods folder.
// It is used for rollback and replacement cleanup, never for logical deletion.
func (h *MealHandler) removeFoodImage(relativePath string) {
	clean := filepath.Clean(filepath.FromSlash(relativePath))
	if clean == "." || clean == "foods" || strings.HasPrefix(clean, "..") ||
		!strings.HasPrefix(clean, "foods"+string(filepath.Separator)) {
		return
	}
	_ = os.Remove(filepath.Join(h.uploadDir, clean))
}

func (h *MealHandler) Foods(c *gin.Context) {
	scope := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("scope", string(service.FoodListScopeActive))))
	if scope == string(service.FoodListScopeDeleted) {
		foods, err := h.service.ListDeletedFoods(c.Request.Context(), currentActor(c), c.Query("keyword"))
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.WriteData(c, http.StatusOK, foods)
		return
	}
	if scope != string(service.FoodListScopeActive) {
		writeBadRequest(c, fmt.Errorf("invalid food scope"))
		return
	}
	foods, err := h.service.ListFoods(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, foods)
}

func (h *MealHandler) Food(c *gin.Context) {
	foodID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	food, err := h.service.ActiveFood(c.Request.Context(), currentActor(c), foodID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, food)
}

func (h *MealHandler) CreateFood(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFoodFormBytes)
	values, err := parseFoodForm(c, false)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	imagePath, err := h.saveFoodImage(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	if imagePath != "" {
		values.iconType = model.FoodIconTypeUpload
		values.iconValue = imagePath
	} else if values.iconType == model.FoodIconTypeUpload {
		// Uploaded paths are server-owned and must never be accepted from form text.
		values.iconValue = ""
	}

	food, err := h.service.CreateFood(c.Request.Context(), currentActor(c), service.CreateFoodInput{
		Name: values.name, CaloriesPer100G: values.caloriesPer100G,
		CarbohydratePer100G: values.carbohydratePer100G, ProteinPer100G: values.proteinPer100G,
		FatPer100G: values.fatPer100G, IconType: values.iconType, IconValue: values.iconValue,
		ConfirmNutritionMismatch: values.confirmNutritionMismatch,
	})
	if err != nil {
		h.removeFoodImage(imagePath)
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusCreated, food)
}

func (h *MealHandler) UpdateFood(c *gin.Context) {
	foodID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	actor := currentActor(c)
	current, err := h.service.ActiveFood(c.Request.Context(), actor, foodID)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFoodFormBytes)
	values, err := parseFoodForm(c, true)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	imagePath, err := h.saveFoodImage(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	if imagePath != "" {
		values.iconType = model.FoodIconTypeUpload
		values.iconValue = imagePath
	} else if values.iconType == model.FoodIconTypeUpload {
		values.iconValue = ""
	}

	updated, err := h.service.UpdateFood(c.Request.Context(), actor, foodID, service.UpdateFoodInput{
		Name: values.name, CaloriesPer100G: values.caloriesPer100G,
		CarbohydratePer100G: values.carbohydratePer100G, ProteinPer100G: values.proteinPer100G,
		FatPer100G: values.fatPer100G, IconType: values.iconType, IconValue: values.iconValue,
		ConfirmNutritionMismatch: values.confirmNutritionMismatch, Version: values.version,
	})
	if err != nil {
		h.removeFoodImage(imagePath)
		writeServiceError(c, err)
		return
	}
	if current.IconType == model.FoodIconTypeUpload && current.IconValue != updated.IconValue {
		h.removeFoodImage(current.IconValue)
	}
	response.WriteData(c, http.StatusOK, updated)
}

func (h *MealHandler) DeleteFood(c *gin.Context) {
	foodID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request versionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	food, err := h.service.DeleteFood(c.Request.Context(), currentActor(c), foodID, request.Version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, food)
}

func (h *MealHandler) RestoreFood(c *gin.Context) {
	foodID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request versionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	food, err := h.service.RestoreFood(c.Request.Context(), currentActor(c), foodID, request.Version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, food)
}

type mealRecordRequest struct {
	MealDate    string         `json:"meal_date" binding:"required"`
	MealType    model.MealType `json:"meal_type" binding:"required"`
	FoodID      uint64         `json:"food_id" binding:"required"`
	WeightGrams float64        `json:"weight_grams" binding:"required"`
	Version     uint64         `json:"version,omitempty"`
}

func parseMealDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("meal_date must use YYYY-MM-DD")
	}
	return date, nil
}

func (h *MealHandler) CreateMyMealRecord(c *gin.Context) {
	var request mealRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	date, err := parseMealDate(request.MealDate)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	record, err := h.service.CreateMyMealRecord(c.Request.Context(), currentActor(c), service.CreateMealRecordInput{
		MealDate: date, MealType: request.MealType, FoodID: request.FoodID, WeightGrams: request.WeightGrams,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusCreated, record)
}

func (h *MealHandler) UpdateMyMealRecord(c *gin.Context) {
	recordID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request mealRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	date, err := parseMealDate(request.MealDate)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	record, err := h.service.UpdateMyMealRecord(c.Request.Context(), currentActor(c), recordID, service.UpdateMealRecordInput{
		MealDate: date, MealType: request.MealType, FoodID: request.FoodID,
		WeightGrams: request.WeightGrams, Version: request.Version,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, record)
}

func (h *MealHandler) DeleteMyMealRecord(c *gin.Context) {
	recordID, err := parseID(c, "id")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var request versionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.service.DeleteMyMealRecord(c.Request.Context(), currentActor(c), recordID, request.Version); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MealHandler) MyMealDay(c *gin.Context) {
	date, err := parseMealDate(c.Query("date"))
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	day, err := h.service.MyMealDay(c.Request.Context(), currentActor(c), date)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, day)
}

func (h *MealHandler) MyMealCalendar(c *gin.Context) {
	calendar, err := h.service.MyMealCalendar(c.Request.Context(), currentActor(c), c.Query("month"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, calendar)
}

func (h *MealHandler) MemberOptions(c *gin.Context) {
	options, err := h.service.MealMemberOptions(c.Request.Context(), currentActor(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, options)
}

func (h *MealHandler) MemberToday(c *gin.Context) {
	memberID, err := parseID(c, "memberID")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	day, err := h.service.MemberToday(c.Request.Context(), currentActor(c), memberID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, day)
}
