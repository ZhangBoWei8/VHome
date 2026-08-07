package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

type PantryHandler struct {
	service   *service.PantryService
	uploadDir string
}

func NewPantryHandler(s *service.PantryService, dir string) *PantryHandler {
	return &PantryHandler{service: s, uploadDir: dir}
}
func (h *PantryHandler) Locations(c *gin.Context) {
	v, e := h.service.Locations(c.Request.Context())
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, v)
}

type locationRequest struct {
	Name        string            `json:"name"`
	IconKey     string            `json:"icon_key"`
	StorageType model.StorageType `json:"storage_type"`
}

func (h *PantryHandler) CreateLocation(c *gin.Context) {
	var q locationRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.CreateLocation(c.Request.Context(), a, q.Name, q.IconKey, q.StorageType)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusCreated, v)
}
func (h *PantryHandler) Templates(c *gin.Context) {
	v, e := h.service.Templates(c.Request.Context())
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, v)
}

type templateRequest struct {
	Name        string   `json:"name"`
	IconKey     string   `json:"icon_key"`
	DefaultUnit string   `json:"default_unit"`
	ColdDays    *int     `json:"cold_days"`
	AmbientDays *int     `json:"ambient_days"`
	Calories    *float64 `json:"calories"`
	Protein     *float64 `json:"protein"`
	Fat         *float64 `json:"fat"`
	Carbs       *float64 `json:"carbs"`
}

func (h *PantryHandler) CreateTemplate(c *gin.Context) {
	var q templateRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.CreateTemplate(c.Request.Context(), a, service.CreateTemplateInput{Name: q.Name, IconKey: q.IconKey, DefaultUnit: q.DefaultUnit, ColdDays: q.ColdDays, AmbientDays: q.AmbientDays, Calories: q.Calories, Protein: q.Protein, Fat: q.Fat, Carbs: q.Carbs})
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusCreated, v)
}
func parseFloatPtr(v string) *float64 {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	x, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return nil
	}
	return &x
}
func parseUintPtr(v string) *uint64 {
	if v == "" {
		return nil
	}
	x, e := strconv.ParseUint(v, 10, 64)
	if e != nil {
		return nil
	}
	return &x
}
func saveImage(c *gin.Context, dir string) (string, error) {
	fh, e := c.FormFile("image")
	if e == http.ErrMissingFile {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	if fh.Size > 5<<20 {
		return "", fmt.Errorf("image too large")
	}
	f, e := fh.Open()
	if e != nil {
		return "", e
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	mime := http.DetectContentType(head[:n])
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mime]
	if ext == "" {
		return "", fmt.Errorf("unsupported image")
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	name := hex.EncodeToString(b) + ext
	rel := filepath.Join("inventory", time.Now().Format("2006/01"), name)
	abs := filepath.Join(dir, rel)
	if e = os.MkdirAll(filepath.Dir(abs), 0755); e != nil {
		return "", e
	}
	if e = c.SaveUploadedFile(fh, abs); e != nil {
		return "", e
	}
	return filepath.ToSlash(rel), nil
}
func inventoryForm(c *gin.Context) (service.InventoryInput, error) {
	q, err := strconv.ParseFloat(c.PostForm("quantity"), 64)
	if err != nil {
		return service.InventoryInput{}, err
	}
	lid, err := strconv.ParseUint(c.PostForm("location_id"), 10, 64)
	if err != nil {
		return service.InventoryInput{}, err
	}
	stock, err := time.Parse("2006-01-02", c.PostForm("stocked_on"))
	if err != nil {
		return service.InventoryInput{}, err
	}
	var exp time.Time
	if c.PostForm("expires_on") != "" {
		exp, err = time.Parse("2006-01-02", c.PostForm("expires_on"))
		if err != nil {
			return service.InventoryInput{}, err
		}
	}
	return service.InventoryInput{TemplateID: parseUintPtr(c.PostForm("template_id")), LocationID: lid, Name: c.PostForm("name"), IconKey: c.PostForm("icon_key"), Quantity: q, Unit: c.PostForm("unit"), StockedOn: stock, ExpiresOn: exp, Calories: parseFloatPtr(c.PostForm("calories")), Protein: parseFloatPtr(c.PostForm("protein")), Fat: parseFloatPtr(c.PostForm("fat")), Carbs: parseFloatPtr(c.PostForm("carbs")), Description: c.PostForm("description")}, nil
}
func (h *PantryHandler) Inventory(c *gin.Context) {
	v, e := h.service.ListInventory(c.Request.Context(), c.DefaultQuery("scope", "active"), c.DefaultQuery("order", "asc"))
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, v)
}
func (h *PantryHandler) CreateInventory(c *gin.Context) {
	in, e := inventoryForm(c)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	path, e := saveImage(c, h.uploadDir)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	in.ImagePath = path
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.CreateInventory(c.Request.Context(), a, in)
	if e != nil {
		if path != "" {
			_ = os.Remove(filepath.Join(h.uploadDir, path))
		}
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusCreated, v)
}
func (h *PantryHandler) UpdateInventory(c *gin.Context) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	version, e := strconv.ParseUint(c.PostForm("version"), 10, 64)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	in, e := inventoryForm(c)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	path, e := saveImage(c, h.uploadDir)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	in.ImagePath = path
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.UpdateInventory(c.Request.Context(), a, id, version, in)
	if e != nil {
		if path != "" {
			_ = os.Remove(filepath.Join(h.uploadDir, path))
		}
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, v)
}

type discardRequest struct {
	Version uint64 `json:"version"`
	Reason  string `json:"reason"`
}

func (h *PantryHandler) Discard(c *gin.Context) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	var q discardRequest
	if e = c.ShouldBindJSON(&q); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	if e = h.service.Discard(c.Request.Context(), a, id, q.Version, q.Reason); e != nil {
		writeServiceError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *PantryHandler) Notifications(c *gin.Context) {
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.Notifications(c.Request.Context(), a)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, v)
}
func (h *PantryHandler) ReadReminder(c *gin.Context) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	if e = h.service.ReadReminder(c.Request.Context(), a, id, model.ReminderMilestone(c.Param("milestone"))); e != nil {
		writeServiceError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
