package operation

import "time"

type Operation struct {
	ID           string     `json:"id"`
	Action       string     `json:"action"`
	Status       string     `json:"status"`
	ProductID    *int64     `json:"product_id"`
	ErrorCode    *string    `json:"error_code"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}
