package schemas

type UserWordsTotal struct {
    User       string `json:"user"`
    TotalWords int    `json:"total_words"`
}

type UserWordsTotalsResponse struct {
    Stats []UserWordsTotal `json:"stats"`
}
