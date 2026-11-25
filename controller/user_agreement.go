package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUserAgreementStatus(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Unauthorized"))
		return
	}
	status, err := model.GetUserAgreementStatus(c.Request.Context(), identity.UserID)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusInternalServerError, common.NewMessage("Please try again later."))
		return
	}
	common.ApiSuccess(c, status)
}

func AcceptUserAgreement(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok || !middleware.IsSameOriginDashboardWrite(c) {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Unauthorized"))
		return
	}
	var body struct {
		Revision string `json:"revision"`
	}
	if err := middleware.ReadDashboardJSON(c, &body); err != nil || body.Revision == "" || len(body.Revision) > 64 {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid request"))
		return
	}
	err := model.AcceptUserAgreement(c.Request.Context(), identity, body.Revision)
	switch {
	case errors.Is(err, model.ErrUserAgreementChanged):
		common.ApiErrorStatus(c, http.StatusConflict, common.NewMessage("The user agreement changed. Review the latest version."), gin.H{"code": "USER_AGREEMENT_UPDATED"})
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, model.ErrUserSessionInactive):
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Unauthorized"))
	case err != nil:
		common.ApiErrorStatus(c, http.StatusInternalServerError, common.NewMessage("Please try again later."))
	default:
		common.ApiSuccess(c, model.UserAgreementStatus{Revision: body.Revision})
	}
}
