package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/http/limit"
	"github.com/goravel/framework/http/middleware"
	"github.com/goravel/framework/support"

	"goravel/app/facades"
	"goravel/app/http/controllers"
	appmiddleware "goravel/app/http/middleware"
	"goravel/app/http/responses"
)

const maximumAPIRequestBytes = 64 * 1024

func Web() {
	registerRateLimiters()
	facades.Route().Recover(func(ctx http.Context, _ any) {
		facades.Log().
			WithContext(ctx).
			With(map[string]any{
				"method": ctx.Request().Method(),
				"path":   ctx.Request().Path(),
			}).
			Error("recovered request panic")
		_ = responses.AbortJSON(ctx, http.StatusInternalServerError, http.Json{
			"error": http.Json{
				"code":    "INTERNAL_ERROR",
				"message": "Hệ thống đang gặp sự cố, vui lòng thử lại sau",
				"details": http.Json{},
			},
		}).Abort()
	})

	facades.Route().Get("/", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("welcome.tmpl", map[string]any{
			"version": support.Version,
		})
	})

	facades.Route().Static("public", "./public")

	healthController := controllers.NewHealthController()
	contentController := controllers.NewContentController()
	commentController := controllers.NewCommentController()
	messageController := controllers.NewMessageController()
	assistantController := controllers.NewAssistantController()

	facades.Route().
		Prefix("/api/v1").
		Middleware(appmiddleware.NewRequestBodyLimit(maximumAPIRequestBytes)).
		Group(func(router route.Router) {
			writeRouter := router.Middleware(middleware.Throttle("api-writes"))
			assistantRouter := router.Middleware(middleware.Throttle("assistant"))

			router.Get("/health", healthController.Show)
			writeRouter.Post("/demo/sessions", contentController.CreateDemoSession)
			router.Get("/users", contentController.ListUsers)
			writeRouter.Put("/users/me", contentController.UpdateProfile)
			router.Get("/topics", contentController.ListTopics)
			router.Get("/posts", contentController.ListPosts)
			writeRouter.Post("/posts", contentController.CreatePost)
			writeRouter.Delete("/posts/{id}", contentController.DeletePost)
			router.Get("/posts/{id}/comments", commentController.List)
			writeRouter.Post("/posts/{id}/comments", commentController.Create)
			writeRouter.Delete(
				"/posts/{id}/comments/{commentId}",
				commentController.Delete,
			)
			writeRouter.Put("/posts/{id}/reaction", contentController.PutReaction)
			writeRouter.Delete("/posts/{id}/reaction", contentController.DeleteReaction)
			router.Get("/messages", messageController.List)
			writeRouter.Post("/messages", messageController.Create)
			assistantRouter.Get(
				"/assistant/conversations",
				assistantController.ListConversations,
			)
			assistantRouter.Get(
				"/assistant/conversations/{id}",
				assistantController.ShowConversation,
			)
			assistantRouter.Post("/assistant/questions", assistantController.Ask)
		})

	facades.Route().Fallback(func(ctx http.Context) http.Response {
		return responses.Error(
			ctx,
			http.StatusNotFound,
			"NOT_FOUND",
			"Không tìm thấy endpoint yêu cầu",
			http.Json{"path": ctx.Request().Path()},
		)
	})
}

func registerRateLimiters() {
	key := func(ctx http.Context) string {
		return ctx.Request().Ip()
	}

	facades.RateLimiter().For("api-writes", func(ctx http.Context) http.Limit {
		return limit.PerMinute(120).
			By(key(ctx)).
			Response(rateLimitedResponse)
	})
	facades.RateLimiter().For("assistant", func(ctx http.Context) http.Limit {
		return limit.PerMinute(20).
			By(key(ctx)).
			Response(rateLimitedResponse)
	})
}

func rateLimitedResponse(ctx http.Context) {
	retryAfter := ctx.Response().Origin().Header().Get(middleware.HeaderRetryAfter)
	_ = responses.AbortJSON(ctx, http.StatusTooManyRequests, http.Json{
		"error": http.Json{
			"code":    "RATE_LIMITED",
			"message": "Bạn đang gửi yêu cầu quá nhanh, vui lòng thử lại sau",
			"details": http.Json{"retryAfterSeconds": retryAfter},
		},
	}).Abort()
}
