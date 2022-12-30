package driveFS

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"os"
	"strings"
)

type apath struct {
	npath string
	names []string
}

func newApath(x string) apath {
	res := []string{}
	for _, x := range strings.Split(strings.ReplaceAll(x, "\\", "/"), "/") {
		if x == "" || x == "." {
			continue
		}
		if x == ".." {
			l := len(res)
			if l > 0 {
				res = res[:l-1]
			}
			continue
		}
		res = append(res, x)
	}

	return apath{strings.Join(res, "/"), res}
}

type UtilHashSize struct {
	Size int64
	Sha1 string
}

func NewUtilHashSize(p string) (*UtilHashSize, error) {
	s, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha1.New()

	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return &UtilHashSize{
		Size: s.Size(),
		Sha1: hex.EncodeToString(h.Sum(nil)),
	}, nil
}
