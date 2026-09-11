package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/service"
)

func writeBadRequest(c *gin.Context, err error) {
	_ = c.Error(err)

	response.WriteError(
		c,
		http.StatusBadRequest,
		"BAD_REQUEST",
		"请求格式不正确",
	)
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMemoTimeInvalid):
		response.WriteError(c, http.StatusUnprocessableEntity, "MEMO_TIME_INVALID", "提醒时间必须晚于当前时间，并以30分钟为单位")

	case errors.Is(err, service.ErrMemoRecipientInvalid):
		response.WriteError(c, http.StatusUnprocessableEntity, "MEMO_RECIPIENT_INVALID", "提醒成员不存在、未启用或不属于当前家庭")

	case errors.Is(err, service.ErrMemoSlotFull):
		response.WriteError(c, http.StatusConflict, "MEMO_SLOT_FULL", "该小时的备忘事项已达到上限")

	case errors.Is(err, service.ErrMemoAlreadyDismissed):
		response.WriteError(c, http.StatusConflict, "MEMO_ALREADY_DISMISSED", "你已经屏蔽了这条提醒")

	case errors.Is(err, service.ErrSecretEncryptionUnavailable):
		response.WriteError(c, http.StatusServiceUnavailable, "SECRET_ENCRYPTION_UNAVAILABLE", "服务器尚未配置通知密钥")

	case errors.Is(err, service.ErrNotificationConfiguration):
		response.WriteError(c, http.StatusUnprocessableEntity, "NOTIFICATION_CONFIGURATION_INCOMPLETE", "通知配置不完整")

	case errors.Is(err, service.ErrSMSProviderUnavailable):
		response.WriteError(c, http.StatusConflict, "SMS_PROVIDER_UNAVAILABLE", "腾讯云短信接口当前仅预留配置，暂未启用")

	case errors.Is(err, service.ErrCalendarNoticeUnavailable):
		response.WriteError(c, http.StatusServiceUnavailable, "CALENDAR_NOTICE_UNAVAILABLE", "尚未找到国务院发布的对应年度节假日通知")

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

	case errors.Is(err, service.ErrFoodNameExists):
		response.WriteError(c, http.StatusConflict, "FOOD_NAME_EXISTS", "已有同名食品，请直接使用已有食品或更换名称")

	case errors.Is(err, service.ErrFoodRestoreRequired):
		response.WriteError(c, http.StatusConflict, "FOOD_RESTORE_REQUIRED", "回收站中已有同名食品，请先恢复该食品")

	case errors.Is(err, service.ErrNutritionConfirmationRequired):
		response.WriteError(c, http.StatusConflict, "NUTRITION_CONFIRMATION_REQUIRED", "热量与三大营养素估算值差异较大，请确认后再保存")

	case errors.Is(err, service.ErrAgentMemoryFull):
		response.WriteError(c, http.StatusConflict, "AGENT_MEMORY_FULL", "记忆库已满，请先删除一些不再需要的记忆")

	case errors.Is(err, service.ErrAgentMemoryScopeInvalid):
		response.WriteError(c, http.StatusUnprocessableEntity, "AGENT_MEMORY_SCOPE_INVALID", "记忆的作用域不合法")

	case errors.Is(err, service.ErrAgentConversationFull):
		response.WriteError(c, http.StatusConflict, "AGENT_CONVERSATION_FULL", "对话数量已达上限，请先删除一些旧对话")

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
