package support

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"
)

var ErrMissingUserID = errors.New("header X-User-ID phải là một UUID hợp lệ")

var ErrInvalidResourceID = errors.New("ID phải là một UUID hợp lệ")

func ParseResourceID(value string) (string, error) {
	candidate := strings.TrimSpace(value)
	id, err := uuid.Parse(candidate)
	if err != nil ||
		id == uuid.Nil ||
		len(candidate) != len(uuid.Nil.String()) ||
		!strings.EqualFold(candidate, id.String()) {
		return "", ErrInvalidResourceID
	}

	return id.String(), nil
}

func ParseUserIDHeader(value string) (string, error) {
	id, err := ParseResourceID(value)
	if err != nil {
		return "", ErrMissingUserID
	}

	return id, nil
}

func CurrentUserID(ctx http.Context) (string, error) {
	return ParseUserIDHeader(ctx.Request().Header("X-User-ID"))
}
