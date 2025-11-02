package schemas

type Group struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
}

type AllowedGroupsResponse struct {
	Groups []Group `json:"groups"`
}
