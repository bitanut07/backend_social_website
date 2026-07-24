package middleware

import (
	nethttp "net/http"
	"strconv"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/http/responses"
)

type RequestBodyLimit struct {
	maxBytes int64
}

func NewRequestBodyLimit(maxBytes int64) *RequestBodyLimit {
	return &RequestBodyLimit{maxBytes: maxBytes}
}

func (m *RequestBodyLimit) Signature() string {
	return "artly:request_body_limit:" + strconv.FormatInt(m.maxBytes, 10)
}

func (m *RequestBodyLimit) Handle(ctx http.Context) {
	request := ctx.Request().Origin()
	if request == nil || request.Body == nil {
		ctx.Request().Next()
		return
	}

	if request.ContentLength > m.maxBytes {
		_ = responses.AbortJSON(ctx, http.StatusRequestEntityTooLarge, http.Json{
			"error": http.Json{
				"code":    "PAYLOAD_TOO_LARGE",
				"message": "Dữ liệu gửi lên vượt quá giới hạn cho phép",
				"details": http.Json{"maxBytes": m.maxBytes},
			},
		}).Abort()
		return
	}

	request.Body = nethttp.MaxBytesReader(
		ctx.Response().Writer(),
		request.Body,
		m.maxBytes,
	)
	ctx.Request().Next()
}
