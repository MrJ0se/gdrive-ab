package driveFS

import (
	"time"
)

type DFile struct {
	Id           string
	Name         string
	Path         string
	TimeMod      time.Time
	Properties   map[string]string
	IsFolder     bool
	Size         int64 //Files only
	Sha1Checksum string
}

func (x *DFile) GarantPropertiesMap() {
	if x.Properties == nil {
		x.Properties = map[string]string{}
	}
}
