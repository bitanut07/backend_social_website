package controllers

import (
	"github.com/goravel/framework/contracts/http"
)

type HealthController struct{}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (r *HealthController) Show(ctx http.Context) http.Response {
	return ctx.Response().Success().Json(http.Json{
		"status": "OK",
	})
}
