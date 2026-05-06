package store

import (
	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

func (s *Store) GetInternetRadioStations() ([]model.InternetRadioStation, error) {
	var stations []model.InternetRadioStation
	err := s.db.Order("name ASC").Find(&stations).Error
	return stations, err
}

func (s *Store) GetInternetRadioStation(id uint) (*model.InternetRadioStation, error) {
	var st model.InternetRadioStation
	if err := s.db.First(&st, id).Error; err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) CreateInternetRadioStation(name, streamURL, homepageURL string) (*model.InternetRadioStation, error) {
	st := model.InternetRadioStation{
		Name:        name,
		StreamURL:   streamURL,
		HomepageURL: homepageURL,
	}
	if err := s.db.Create(&st).Error; err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) UpdateInternetRadioStation(id uint, name, streamURL, homepageURL string) error {
	var existing model.InternetRadioStation
	if err := s.db.First(&existing, id).Error; err != nil {
		return err
	}
	return s.db.Model(&existing).Updates(map[string]any{
		"name":         name,
		"stream_url":   streamURL,
		"homepage_url": homepageURL,
	}).Error
}

func (s *Store) DeleteInternetRadioStation(id uint) error {
	res := s.db.Delete(&model.InternetRadioStation{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) UpdateInternetRadioStationCoverPath(id uint, path string) error {
	var existing model.InternetRadioStation
	if err := s.db.First(&existing, id).Error; err != nil {
		return err
	}
	return s.db.Model(&existing).Update("cover_path", path).Error
}
