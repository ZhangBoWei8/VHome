package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/service"
)

func writeBadRequest(
	c *gin.Context,
	err error,
) {
	_ = c.Error(err)

	response.WriteError(
		c,
		http.StatusBadRequest,
		"BAD_REQUEST",
		"请求格式不正确",
	)
}

func writeServiceError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		response.WriteError(
			c,
			http.StatusUnprocessableEntity,
			"VALIDATION_FAILED",
			"请求参数不合法",
		)

	case errors.Is(err, service.ErrAlreadyInitialized):
		response.WriteError(
			c,
			http.StatusConflict,
			"SYSTEM_ALREADY_INITIALIZED",
			"家庭已经完成初始化",
		)

	case errors.Is(err, service.ErrNotInitialized):
		response.WriteError(
			c,
			http.StatusConflict,
			"SYSTEM_NOT_INITIALIZED",
			"系统尚未初始化",
		)

	case errors.Is(err, service.ErrRegistrationDisabled):
		response.WriteError(
			c,
			http.StatusForbidden,
			"REGISTRATION_DISABLED",
			"家庭当前未开放成员注册",
		)

	case errors.Is(
		err,
		service.ErrInvalidHouseholdPassword,
	):
		response.WriteError(
			c,
			http.StatusForbidden,
			"INVALID_HOUSEHOLD_PASSWORD",
			"家庭密码不正确",
		)

	case errors.Is(err, service.ErrUsernameAlreadyExists):
		response.WriteError(
			c,
			http.StatusConflict,
			"USERNAME_ALREADY_EXISTS",
			"该成员名称已被使用",
		)

	case errors.Is(err, service.ErrInvalidCredentials):
		response.WriteError(
			c,
			http.StatusUnauthorized,
			"INVALID_CREDENTIALS",
			"用户名或密码错误",
		)

	case errors.Is(
		err,
		service.ErrMemberPendingApproval,
	):
		response.WriteError(
			c,
			http.StatusForbidden,
			"MEMBER_PENDING_APPROVAL",
			"成员申请正在等待审批",
		)

	case errors.Is(err, service.ErrMemberRejected):
		response.WriteError(
			c,
			http.StatusForbidden,
			"MEMBER_REJECTED",
			"成员申请未通过审批",
		)

	case errors.Is(err, service.ErrMemberDisabled):
		response.WriteError(
			c,
			http.StatusForbidden,
			"MEMBER_DISABLED",
			"成员账号已被禁用",
		)

	case errors.Is(err, service.ErrUnauthenticated):
		response.WriteError(
			c,
			http.StatusUnauthorized,
			"UNAUTHORIZED",
			"登录状态无效或已经过期",
		)

	case errors.Is(err, service.ErrInvalidCSRFToken):
		response.WriteError(
			c,
			http.StatusForbidden,
			"INVALID_CSRF_TOKEN",
			"CSRF Token 无效",
		)

	case errors.Is(err, service.ErrForbidden):
		response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "没有执行该操作的权限")

	case errors.Is(err, service.ErrNotFound):
		response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "资源不存在")

	case errors.Is(err, service.ErrMaterialNameExists):
		response.WriteError(c, http.StatusConflict, "MATERIAL_NAME_EXISTS", "已有同名物料，请直接使用已有品类或更换名称")

	case errors.Is(err, service.ErrConflict):
		response.WriteError(c, http.StatusConflict, "VERSION_CONFLICT", "数据已经发生变化，请刷新后重试")

	default:
		_ = c.Error(err)

		response.WriteError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"服务暂时不可用",
		)
	}
}
