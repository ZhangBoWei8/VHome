package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/middleware"
	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

type MemberHandler struct{ service *service.IdentityService }

func NewMemberHandler(s *service.IdentityService) *MemberHandler { return &MemberHandler{service: s} }
func memberID(c *gin.Context) (uint64, bool) {
	v, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		writeBadRequest(c, e)
		return 0, false
	}
	return v, true
}
func (h *MemberHandler) List(c *gin.Context) {
	a, ok := middleware.CurrentIdentity(c)
	if !ok {
		return
	}
	v, e := h.service.ListMembers(c.Request.Context(), a)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	out := make([]MemberData, 0, len(v))
	for _, m := range v {
		out = append(out, newMemberData(m))
	}
	response.WriteData(c, http.StatusOK, out)
}

type versionRequest struct {
	Version uint64 `json:"version" binding:"required"`
}

func (h *MemberHandler) review(c *gin.Context, approve bool) {
	id, ok := memberID(c)
	if !ok {
		return
	}
	var req versionRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.ReviewMember(c.Request.Context(), a, id, req.Version, approve)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, newMemberData(v))
}
func (h *MemberHandler) Approve(c *gin.Context) { h.review(c, true) }
func (h *MemberHandler) Reject(c *gin.Context)  { h.review(c, false) }

type roleRequest struct {
	Role    model.MemberRole `json:"role" binding:"required"`
	Version uint64           `json:"version" binding:"required"`
}

func (h *MemberHandler) Role(c *gin.Context) {
	id, ok := memberID(c)
	if !ok {
		return
	}
	var req roleRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.ChangeMemberRole(c.Request.Context(), a, id, req.Version, req.Role)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, newMemberData(v))
}
func (h *MemberHandler) Disable(c *gin.Context) {
	id, ok := memberID(c)
	if !ok {
		return
	}
	var req versionRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		writeBadRequest(c, e)
		return
	}
	a, _ := middleware.CurrentIdentity(c)
	v, e := h.service.DisableMember(c.Request.Context(), a, id, req.Version)
	if e != nil {
		writeServiceError(c, e)
		return
	}
	response.WriteData(c, http.StatusOK, newMemberData(v))
}
