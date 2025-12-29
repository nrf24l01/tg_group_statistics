package update

import "time"

type Date struct {
	Date time.Time
}

type UserStats struct {
	MessagesPerDay map[string]int  // key: dd-mm-yyyy
	TotalMessages int
	WordCountsPerDay map[string]map[string]int64 // key: dd-mm-yyyy -> word -> count
}

type GroupStats struct {
	MessagesPerDay map[string]int  // key: dd-mm-yyyy
	TotalMessages  int
	WordCountsPerDay map[string]map[string]int64 // key: dd-mm-yyyy -> word -> count
}