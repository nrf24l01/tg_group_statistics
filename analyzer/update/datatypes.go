package update

import "time"

type Date struct {
	Date time.Time
}

type UserStat struct {
	MessagesPerDay map[string]int  // key: dd-mm-yyyy
	TotalMessages int
}