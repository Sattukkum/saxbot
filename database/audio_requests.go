package database

func (p *PostgresRepository) SaveAudio(audio *Audio) error {
	return p.db.Create(audio).Error
}

func (p *PostgresRepository) GetAudio(fileID string) (Audio, error) {
	var audio Audio
	err := p.db.Where("file_id = ?", fileID).First(&audio).Error
	if err != nil {
		return Audio{}, err
	}
	return audio, nil
}

func (p *PostgresRepository) GetAudioByUniqueID(uniqueID string) (Audio, error) {
	var audio Audio
	err := p.db.Where("unique_id = ?", uniqueID).First(&audio).Error
	if err != nil {
		return Audio{}, err
	}
	return audio, nil
}

func (p *PostgresRepository) GetAudioByAlbumIDAndTrackNumber(albumID int, trackNumber int) (Audio, error) {
	var audio Audio
	err := p.db.Where("album_id = ? AND track_number = ?", albumID, trackNumber).First(&audio).Error
	if err != nil {
		return Audio{}, err
	}
	return audio, nil
}

func (p *PostgresRepository) GetAudioByName(name string) (Audio, error) {
	var audio Audio
	err := p.db.Where("name = ?", name).First(&audio).Error
	if err != nil {
		return Audio{}, err
	}
	return audio, nil
}
