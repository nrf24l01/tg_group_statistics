package schemas

import "time"

type UserStatsPerDay struct {
	TotalMessages int `json:"total_messages"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	Day           time.Time `json:"day"`
}

type MessagesPerUserPerDayResponse struct {
	Stats []UserStatsPerDay `json:"stats"`
}

type DayTotal struct {
	Day      string `json:"day"`
	TotalMsg int    `json:"total_msg"`
}

type DayTotalsResponse struct {
	Stats []DayTotal `json:"stats"`
}

type UserTotal struct {
	User     string `json:"user"`
	TotalMsg int    `json:"total_msg"`
}

type UserTotalsResponse struct {
	Stats []UserTotal `json:"stats"`
}