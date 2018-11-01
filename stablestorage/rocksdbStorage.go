package stablestorage

type RocksdbStorage struct {
	dbpath string
}

func (rs *RocksdbStorage) GetWAL(clusterId, nodeId uint64) (*WAL, error) {
	return nil, nil
}
func (rs *RocksdbStorage) GetSnapshooter(clusterId, nodeId uint64) *Snapshotter{
	return nil
}

func (rs *RocksdbStorage) ExistWAL(clusterId, nodeId uint64) bool {
	return false
}

func NewRocksdbStorage(dbpath string) (*RocksdbStorage, error) {
	return nil, nil
}