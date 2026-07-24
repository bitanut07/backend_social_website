package responses

import "github.com/goravel/framework/contracts/http"

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int64 `json:"totalPages"`
}

func Data(ctx http.Context, status int, value any) http.Response {
	return JSON(ctx, status, http.Json{"data": value})
}

func Paginated(ctx http.Context, value any, pagination Pagination) http.Response {
	return JSON(ctx, http.StatusOK, http.Json{
		"data":       value,
		"pagination": pagination,
	})
}

func Error(ctx http.Context, status int, code string, message string, details any) http.Response {
	return JSON(ctx, status, http.Json{
		"error": http.Json{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}

func JSON(ctx http.Context, status int, value any) http.Response {
	return prepareJSONResponse(ctx).Json(status, value)
}

func AbortJSON(ctx http.Context, status int, value any) http.AbortableResponse {
	return prepareJSONResponse(ctx).Json(status, value)
}

func prepareJSONResponse(ctx http.Context) http.ContextResponse {
	response := ctx.Response()
	response.Header("Cache-Control", "no-store, private")
	response.Header("X-Content-Type-Options", "nosniff")
	response.Origin().Header().Add("Vary", "X-User-ID")
	return response
}

func Page(totalItems int64, page, pageSize int) Pagination {
	totalPages := int64(0)
	if pageSize > 0 {
		totalPages = (totalItems + int64(pageSize) - 1) / int64(pageSize)
	}

	return Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}
