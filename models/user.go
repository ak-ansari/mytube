package models

import "github.com/google/uuid"

type UserStatus string

var (
	Active   UserStatus = "active"
	Inactive UserStatus = "inactive"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	ChannelName string     `json:"channel_name,omitempty"`
	UserName    string     `json:"user_name"`
	Subscribers int        `json:"subscribers"`
	Avatar      string     `json:"avatar,omitempty"`
	Status      UserStatus `json:"status,omitempty"`
}
