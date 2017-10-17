package diecast

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

var MountHaltErr = errors.New(`mount halted`)

type Mount interface {
	Open(string) (http.File, error)
	OpenWithType(string, *http.Request, io.Reader) (http.File, string, error)
	WillRespondTo(string, *http.Request, io.Reader) bool
	GetMountPoint() string
}

func NewMountFromSpec(spec string) (Mount, error) {
	parts := strings.SplitN(spec, `:`, 2)
	var mountPoint string
	var source string
	var scheme string

	if len(parts) == 1 {
		mountPoint = parts[0]
		source = parts[0]
	} else {
		mountPoint = parts[0]
		source = parts[1]
	}

	sourceParts := strings.SplitN(source, `:`, 2)

	if len(sourceParts) == 2 {
		scheme = sourceParts[0]
	}

	var mount Mount

	switch scheme {
	case `http`, `https`:
		mount = &ProxyMount{
			URL:        source,
			MountPoint: mountPoint,
		}

	default:
		if absPath, err := filepath.Abs(source); err == nil {
			source = absPath
		} else {
			return nil, err
		}

		mount = &FileMount{
			Path:       source,
			MountPoint: mountPoint,
		}
	}

	log.Debugf("Creating mount %T: %+v", mount, mount)

	return mount, nil
}

func IsHardStop(err error) bool {
	if err.Error() == `mount halted` {
		return true
	}

	return false
}
