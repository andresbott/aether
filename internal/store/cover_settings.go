package store

import (
	"errors"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// coverSettingsID is the fixed primary key of the CoverSettings singleton row.
const coverSettingsID = 1

// GetCoverSettings returns the generated-cover config singleton. On first call
// (no row yet) it seeds and persists a row with both sets equal to seedStyles,
// so a fresh install generates and offers everything. seedStyles is ignored
// once a row exists.
func (s *Store) GetCoverSettings(seedStyles []string) (model.CoverSettings, error) {
	var cs model.CoverSettings
	err := s.db.First(&cs, coverSettingsID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cs = model.CoverSettings{
			ID:              coverSettingsID,
			DefaultStyles:   append([]string(nil), seedStyles...),
			AvailableStyles: append([]string(nil), seedStyles...),
		}
		if err := s.db.Create(&cs).Error; err != nil {
			return model.CoverSettings{}, err
		}
		return cs, nil
	}
	if err != nil {
		return model.CoverSettings{}, err
	}
	return cs, nil
}

// SetCoverSettings upserts the singleton (always row 1).
func (s *Store) SetCoverSettings(cs model.CoverSettings) error {
	cs.ID = coverSettingsID
	return s.db.Save(&cs).Error
}
