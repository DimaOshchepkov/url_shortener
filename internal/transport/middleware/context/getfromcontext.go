package getcontext

import (
	"context"
	"errors"
)

type (
	key string
)

var (
	ErrKey     key = "errorkey"
	UidKey     key = "uidkey"
	AppIDKey   key = "appidkey"
	IsAdminKey key = "isadminkey"
)

var (
	ErrFailedIsAdminCheck = errors.New("failed to check if user is admin")
)

func APPIDFromContext(ctx context.Context) (uint64, bool) {
	appid, ok := ctx.Value(AppIDKey).(uint64)
	return appid, ok
}

func UIDFromContext(ctx context.Context) (uint64, bool) {
	uid, ok := ctx.Value(UidKey).(uint64)
	return uid, ok
}

func IsAdminFromContext(ctx context.Context) (bool, bool) {
	uid, ok := ctx.Value(IsAdminKey).(bool)
	return uid, ok
}

func ErrorFromContext(ctx context.Context) (error, bool) {
	err, ok := ctx.Value(ErrKey).(error)
	return err, ok
}
