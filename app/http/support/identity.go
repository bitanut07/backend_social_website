package support

import (
	"errors"
	"strconv"
	"strings"

	"github.com/goravel/framework/contracts/http"
)

var ErrMissingUserID = errors.New("header X-User-ID phải là một số nguyên dương")

func ParseUserIDHeader(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrMissingUserID
	}

	return id, nil
}

func CurrentUserID(ctx http.Context) (int64, error) {
	return ParseUserIDHeader(ctx.Request().Header("X-User-ID"))
}
