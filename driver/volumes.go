package driver

type Volume struct {
	id      DiskIdentifier
	options any
}

func (v *Volume) Id() DiskIdentifier {
	return v.id
}

type cephVolumeOpts struct {
	pool string
	name string
}

func NewCephVolume(id DiskIdentifier, pool string, name string) Volume {
	return Volume{
		id: id,
		options: cephVolumeOpts{
			pool: pool,
			name: name,
		},
	}
}

type fileVolumeOpts struct {
	path     string
	readonly bool
}

func NewFileVolume(id DiskIdentifier, path string, readonly bool) Volume {
	return Volume{
		id: id,
		options: fileVolumeOpts{
			path:     path,
			readonly: readonly,
		},
	}
}
