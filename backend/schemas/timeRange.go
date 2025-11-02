package schemas

type TimeRangeQuery struct {
	StartTimestamp uint64 `query:"start" validate:"required"`
	EndTimestamp   uint64 `query:"end" validate:"required,gtfield=StartTimestamp"`
	ChatID		   int64  `query:"chat_id" validate:"required,chatid_allowed"`
}