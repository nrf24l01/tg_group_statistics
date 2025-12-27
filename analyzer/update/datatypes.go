package update

import "time"

type Date struct {
	Date time.Time
}

type UserStats struct {
	MessagesPerDay   map[string]int              // key: dd-mm-yyyy
	WordCountsPerDay map[string]map[string]int64 // key: dd-mm-yyyy -> word -> count
	TotalMessages    int
}

type GroupStats struct {
	MessagesPerDay   map[string]int              // key: dd-mm-yyyy
	WordCountsPerDay map[string]map[string]int64 // key: dd-mm-yyyy -> word -> count
	TotalMessages    int
}