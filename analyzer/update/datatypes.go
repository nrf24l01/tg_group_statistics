package update

import "time"

type Date struct {
	Date time.Time
}

type UserStats struct {
	MessagesPerDay map[string]int  // key: dd-mm-yyyy
	TotalMessages int
}

type GroupStats struct {
	MessagesPerDay map[string]int  // key: dd-mm-yyyy
	TotalMessages  int
}