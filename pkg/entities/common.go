package entities

// Includes additional data with primary data
type MetaData struct {
	Total       int `json:"total,omitempty"`
	PerPage     int `json:"per_page,omitempty"`
	CurrentPage int `json:"current_page,omitempty"`
	Next        int `json:"next,omitempty"`
	Prev        int `json:"prev,omitempty"`
}

// Pagination meta data

type PaginationMetaData struct {
	Size     int    `json:"size"`
	PageSize int    `json:"page_size,omitempty"`
	Next     string `json:"next"`
	Prev     string `json:"prev"`
}

// Common response variable with Pagination
type Response struct {
	StatusCode         int                 `json:"status_code"`
	Message            string              `json:"message"`
	MetaData           *MetaData           `json:"meta_data,omitempty"`
	PaginationMetaData *PaginationMetaData `json:"pagination_meta_data,omitempty"`
	Data               interface{}         `json:"data,omitempty"`
}

type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message"`
}
type GlobalStatistics struct {
	Users             int `json:"users"`
	NotificationsSent int `json:"notifications_sent"`
	Channels          int `json:"channels"`
}

type Pagination struct {
	PageSize  int
	NextToken []byte
}
