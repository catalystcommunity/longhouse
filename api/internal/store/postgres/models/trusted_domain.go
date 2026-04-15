package models

import "time"

type TrustedDomain struct {
	TrustedDomainID string    `gorm:"column:trusted_domain_id;primaryKey" json:"trusted_domain_id"`
	HouseID         string    `gorm:"column:house_id;not null" json:"house_id"`
	Domain          string    `gorm:"column:domain;not null" json:"domain"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (TrustedDomain) TableName() string { return "trusted_domains" }
