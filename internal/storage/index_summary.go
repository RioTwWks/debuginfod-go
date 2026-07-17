package storage

// IndexSummary — текущее состояние индекса для Web UI.
type IndexSummary struct {
	ArtifactsTotal      int64 `json:"artifacts_total"`
	ArtifactsExecutable int64 `json:"artifacts_executable"`
	ArtifactsDebuginfo  int64 `json:"artifacts_debuginfo"`
	ScannedFilesTotal   int64 `json:"scanned_files_total"`
	BytesOnDisk         int64 `json:"bytes_on_disk"`
}

// IndexSummary возвращает сводку индекса и суммарный размер файлов на диске.
func (s *Storage) IndexSummary() (IndexSummary, error) {
	st, err := s.Stats()
	if err != nil {
		return IndexSummary{}, err
	}
	bytes, err := s.IndexedBytesOnDisk()
	if err != nil {
		return IndexSummary{}, err
	}
	return IndexSummary{
		ArtifactsTotal:      st.ArtifactsTotal,
		ArtifactsExecutable: st.ArtifactsExecutable,
		ArtifactsDebuginfo:  st.ArtifactsDebuginfo,
		ScannedFilesTotal:   st.ScannedFilesTotal,
		BytesOnDisk:         bytes,
	}, nil
}

// IndexedBytesOnDisk суммирует размер файлов артефактов с file_path на диске.
func (s *Storage) IndexedBytesOnDisk() (int64, error) {
	rows, err := s.db.Query(rebind(`
		SELECT file_path FROM artifacts WHERE file_path != ''
	`, s.dialect))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var total int64
	seen := make(map[string]struct{})
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return 0, err
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		size, err := statFileSize(path)
		if err != nil {
			continue
		}
		total += size
	}
	return total, rows.Err()
}
