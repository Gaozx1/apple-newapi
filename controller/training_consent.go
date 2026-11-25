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

func GetTrainingConsent(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Unauthorized"))
		return
	}
	consent, err := model.GetTrainingConsent(c.Request.Context(), identity.UserID)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusInternalServerError, common.NewMessage("Please try again later."))
		return
	}
	common.ApiSuccess(c, gin.H{"decision": consent.Decision()})
}

func UpdateTrainingConsent(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Unauthorized"))
		return
	}
	if !middleware.IsSameOriginDashboardWrite(c) {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("Unauthorized"))
		return
	}
	var request struct {
		Decision string `json:"decision"`
	}
	if middleware.ReadDashboardJSON(c, &request) != nil ||
		(request.Decision != model.TrainingAccepted && request.Decision != model.TrainingDeclined) {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Choose whether to allow training data collection."))
		return
	}
	consent, err := model.SetTrainingConsent(c.Request.Context(), identity, request.Decision == model.TrainingAccepted)
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, model.ErrUserSessionInactive) {
		common.ApiErrorStatus(c, http.StatusUnauthorized, common.NewMessage("Unauthorized"))
		return
	}
	if err != nil {
		common.ApiErrorStatus(c, http.StatusInternalServerError, common.NewMessage("Please try again later."))
		return
	}
	common.ApiSuccess(c, gin.H{"decision": consent.Decision()})
}
