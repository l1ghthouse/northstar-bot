package nsserver

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NSServer struct {
	ID                   uuid.UUID         `json:"id,omitempty" gorm:"type:uuid;primary_key;"`
	Name                 string            `json:"name" gorm:"not null;default:null"`
	Region               string            `json:"region" gorm:"not null;default:null"`
	Pin                  string            `json:"pin" gorm:"not null;default:null"`
	RequestedBy          string            `json:"requestedBy" gorm:"not null;default:null"`
	DefaultPassword      string            `json:"defaultPassword" gorm:"not null;default:null"`
	Insecure             bool              `json:"insecure" gorm:"not null;default:false"`
	MainIP               string            `json:"mainIP" gorm:""`
	GameUDPPort          int               `json:"gameUDPPort" gorm:"not null;default:0"`
	AuthTCPPort          int               `json:"authTCPPort" gorm:"not null;default:0"`
	MasterServer         string            `json:"masterServer" gorm:"not null;default:null"`
	OptimizedServerFiles bool              `json:"optimizedServerFiles" gorm:"not null;default:false"`
	ServerVersion        string            `json:"serverVersion" gorm:"not null;default:null"`
	DockerImageVersion   string            `json:"dockerImageVersion" gorm:"not null;default:null"`
	Options              datatypes.JSONMap `json:"options" gorm:""`
	CreatedAt            time.Time
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
