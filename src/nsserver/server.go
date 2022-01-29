package nsserver

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type NSServer struct {
	ID          uuid.UUID `json:"id,omitempty" gorm:"type:uuid;primary_key;"`
	Name        string    `json:"name" gorm:"not null;default:null"`
	Region      string    `json:"region" gorm:"not null;default:null"`
	Pin         *int      `json:"pin" gorm:"not null;default:null"`
	RequestedBy string    `json:"requestedBy" gorm:"not null;default:null"`
	CreatedAt   time.Time
}

func (p *NSServer) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		u, err := uuid.NewV4()
		if err != nil {
			return fmt.Errorf("failed to create uuid: %w", err)
		}
		p.ID = u
	}
	return nil
}
