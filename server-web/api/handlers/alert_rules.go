package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"server-web/model"
)

const maxAlertRuleExprLength = 2048

var forbiddenAlertRuleExprTerms = []string{
	"admin_api",
	"scrape_interval",
	"scrape_duration",
}

type alertRuleRequest struct {
	Name        string `json:"name"`
	Expr        string `json:"expr"`
	Duration    string `json:"duration"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

func (h *Handler) ListAlertRules(c *gin.Context) {
	db, ok := h.requireDB(c)
	if !ok {
		return
	}

	var rules []model.AlertRule
	if err := db.WithContext(c.Request.Context()).Order("id ASC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response{Status: "error", Error: "list alert rules failed"})
		return
	}
	c.JSON(http.StatusOK, response{Status: "success", Data: rules})
}

func (h *Handler) GetAlertRule(c *gin.Context) {
	db, ok := h.requireDB(c)
	if !ok {
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	rule, ok := h.findAlertRule(c, db, id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, response{Status: "success", Data: rule})
}

func (h *Handler) CreateAlertRule(c *gin.Context) {
	db, ok := h.requireDB(c)
	if !ok {
		return
	}

	var req alertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: "invalid alert rule request"})
		return
	}
	rule, ok := normalizeAlertRuleRequest(c, req)
	if !ok {
		return
	}

	if err := db.WithContext(c.Request.Context()).Create(&rule).Error; err != nil {
		c.JSON(http.StatusConflict, response{Status: "error", Error: "create alert rule failed"})
		return
	}
	c.JSON(http.StatusCreated, response{Status: "success", Data: rule})
}

func (h *Handler) UpdateAlertRule(c *gin.Context) {
	db, ok := h.requireDB(c)
	if !ok {
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req alertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: "invalid alert rule request"})
		return
	}
	updated, ok := normalizeAlertRuleRequest(c, req)
	if !ok {
		return
	}

	rule, ok := h.findAlertRule(c, db, id)
	if !ok {
		return
	}
	rule.Name = updated.Name
	rule.Expr = updated.Expr
	rule.Duration = updated.Duration
	rule.Severity = updated.Severity
	rule.Summary = updated.Summary
	rule.Description = updated.Description
	rule.Enabled = updated.Enabled

	if err := db.WithContext(c.Request.Context()).Save(&rule).Error; err != nil {
		c.JSON(http.StatusConflict, response{Status: "error", Error: "update alert rule failed"})
		return
	}
	c.JSON(http.StatusOK, response{Status: "success", Data: rule})
}

func (h *Handler) DeleteAlertRule(c *gin.Context) {
	db, ok := h.requireDB(c)
	if !ok {
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if _, ok := h.findAlertRule(c, db, id); !ok {
		return
	}
	if err := db.WithContext(c.Request.Context()).Delete(&model.AlertRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response{Status: "error", Error: "delete alert rule failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) findAlertRule(c *gin.Context, db *gorm.DB, id uint64) (model.AlertRule, bool) {
	var rule model.AlertRule
	err := db.WithContext(c.Request.Context()).First(&rule, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response{Status: "error", Error: "alert rule not found"})
		return model.AlertRule{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Status: "error", Error: "query alert rule failed"})
		return model.AlertRule{}, false
	}
	return rule, true
}

func normalizeAlertRuleRequest(c *gin.Context, req alertRuleRequest) (model.AlertRule, bool) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 128 {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: "name must be 1-128 characters"})
		return model.AlertRule{}, false
	}

	expr := strings.TrimSpace(req.Expr)
	if err := validateAlertRuleExpr(expr); err != nil {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: err.Error()})
		return model.AlertRule{}, false
	}

	duration := strings.TrimSpace(req.Duration)
	if duration == "" {
		duration = "2m"
	}
	if err := validateAlertRuleDuration(duration); err != nil {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: err.Error()})
		return model.AlertRule{}, false
	}

	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "warning"
	}
	if _, ok := validAlertEventSeverities[severity]; !ok {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: "invalid severity"})
		return model.AlertRule{}, false
	}

	summary := strings.TrimSpace(req.Summary)
	if len(summary) > 512 {
		c.JSON(http.StatusBadRequest, response{Status: "error", Error: "summary must be at most 512 characters"})
		return model.AlertRule{}, false
	}
	description := strings.TrimSpace(req.Description)

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	return model.AlertRule{
		Name:        name,
		Expr:        expr,
		Duration:    duration,
		Severity:    severity,
		Summary:     summary,
		Description: description,
		Enabled:     enabled,
	}, true
}

func validateAlertRuleExpr(expr string) error {
	if expr == "" {
		return errors.New("expr is required")
	}
	if len(expr) > maxAlertRuleExprLength {
		return errors.New("expr too long")
	}

	lower := strings.ToLower(expr)
	for _, term := range forbiddenAlertRuleExprTerms {
		if strings.Contains(lower, term) {
			return errors.New("expr contains forbidden reference")
		}
	}
	if containsPromQLSubquery(expr) {
		return errors.New("expr subquery is not allowed")
	}
	return nil
}

func validateAlertRuleDuration(duration string) error {
	if duration == "" {
		return errors.New("duration must be a positive prometheus duration")
	}
	for i := 0; i < len(duration); {
		start := i
		for i < len(duration) && duration[i] >= '0' && duration[i] <= '9' {
			i++
		}
		if start == i {
			return errors.New("duration must be a positive prometheus duration")
		}
		if !hasNonZeroDigit(duration[start:i]) {
			return errors.New("duration must be a positive prometheus duration")
		}

		unitStart := i
		if i < len(duration) && duration[i] == 'm' {
			i++
			if i < len(duration) && duration[i] == 's' {
				i++
			}
		} else if i < len(duration) && strings.ContainsRune("smhdwy", rune(duration[i])) {
			i++
		}
		if unitStart == i {
			return errors.New("duration must be a positive prometheus duration")
		}
	}
	return nil
}

func hasNonZeroDigit(value string) bool {
	for _, ch := range value {
		if ch >= '1' && ch <= '9' {
			return true
		}
	}
	return false
}

func containsPromQLSubquery(expr string) bool {
	for start := strings.Index(expr, "["); start >= 0; {
		end := strings.Index(expr[start:], "]")
		if end < 0 {
			return false
		}
		inside := expr[start+1 : start+end]
		if strings.Contains(inside, ":") {
			return true
		}
		next := start + end + 1
		if next >= len(expr) {
			return false
		}
		remaining := expr[next:]
		offset := strings.Index(remaining, "[")
		if offset < 0 {
			return false
		}
		start = next + offset
	}
	return false
}
