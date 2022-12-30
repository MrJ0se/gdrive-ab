package driveSync

import (
	"gdriveAB/driveFS"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

/*
obs.: the b.date is a properties with local mod.datetime stored in files,
in folders is used just on the main folder of repo to difer between a sync up/down
prefer utc datetime

process
- get DRepo of local and drive
obs.: rules must be applied to filter local files
if (local.main==drive.main) synced, end!
- generate DiffMap
if (local.main > drive.main) for sync up
else for sync down
- upload/download diff files
- delete diff files

*/

const (
	DSyncUp int = iota
	DSyncDown
	DSyncAlready
	DSyncBlockedUP
	DSyncBlockedDown
)

type DRepoDate struct {
	Main  time.Time
	Files map[string]time.Time
}
type DRepoDiff struct {
	Copy   []string
	Delete []string
}
type DRepoFilter interface {
	FilterFile(fpath string) bool
	FilterFolder(fpath string) bool
}

func simplifyPath(current string, rootpath *string) string {
	if strings.HasPrefix(current, *rootpath) {
		n := strings.ReplaceAll(current[len(*rootpath):], "\\", "/")
		if strings.HasPrefix(n, "/") {
			n = n[1:]
		}
		if strings.HasSuffix(n, "/") {
			n = n[:len(n)-1]
		}
		return n
	}
	panic("broken relative path simplifyPath:" + current + ":" + *rootpath)
	//return current
}

func dRepoDate_localFill(ref *DRepoDate, file fs.FileInfo, filepath string, rootpath *string, filter *DRepoFilter) {
	if file.IsDir() {
		if filter != nil && !(*filter).FilterFolder(simplifyPath(filepath, rootpath)) {
			return
		}
		subs, err := os.ReadDir(filepath)
		if err != nil {
			panic("NewDRepoDate_drive:ReadDirError" + err.Error())
		}
		for _, c := range subs {
			s, e := c.Info()
			if e != nil {
				panic(e)
			}
			dRepoDate_localFill(ref, s, path.Join(filepath, c.Name()), rootpath, filter)
		}
		return
	}
	spath := simplifyPath(filepath, rootpath)
	if filter != nil && !(*filter).FilterFile(spath) {
		return
	}
	ref.Files[spath] = file.ModTime().UTC()
}
func NewDRepoDate_local(root string, filter *DRepoFilter) DRepoDate {
	root = path.Join(root)
	r := DRepoDate{}
	r.Files = make(map[string]time.Time)
	s, err := os.Stat(root)
	if err == nil && s != nil {
		if !s.IsDir() {
			panic("NewDRepoDate_local:IsNotAFolder")
		}
		r.Main = s.ModTime().UTC()
		dRepoDate_localFill(&r, s, root, &root, filter)
	} else {
		r.Main = time.UnixMilli(0)
	}
	return r
}
func dRepoDate_driveGetTime(f *driveFS.DFile) time.Time {
	if f.Properties != nil {
		tv, te := f.Properties["dab-time"]
		if te {
			tnv, tne := strconv.ParseInt(tv, 10, 64)
			if tne == nil {
				return time.UnixMilli(tnv).UTC()
			}
		}
	}
	return time.UnixMilli(0)
}
func dRepoDate_driveFill(ref *DRepoDate, dfs *driveFS.DFS, file *driveFS.DFile, rootpath *string, filter *DRepoFilter) {
	if file.IsFolder {
		if filter != nil && !(*filter).FilterFolder(simplifyPath(file.Path, rootpath)) {
			return
		}
		subs, err := dfs.ReadDir(file.Path)
		if err != nil {
			panic("NewDRepoDate_drive:ReadDirError" + err.Error())
		}
		for _, c := range subs {
			dRepoDate_driveFill(ref, dfs, &c, rootpath, filter)
		}
		return
	}
	spath := simplifyPath(file.Path, rootpath)
	if filter != nil && !(*filter).FilterFile(spath) {
		return
	}
	ref.Files[spath] = dRepoDate_driveGetTime(file)
}
func NewDRepoDate_drive(dfs *driveFS.DFS, root string, filter *DRepoFilter) DRepoDate {
	r := DRepoDate{}
	r.Files = make(map[string]time.Time)
	f, err := dfs.File(root)
	if err == nil && f != nil {
		if !f.IsFolder {
			panic("NewDRepoDate_drive:IsNotAFolder")
		}
		r.Main = dRepoDate_driveGetTime(f)
		dRepoDate_driveFill(&r, dfs, f, &f.Path, filter)
	} else {
		r.Main = time.UnixMilli(0)
	}
	return r
}
func DiffSyncType(local DRepoDate, drive DRepoDate, acceptUp bool, acceptDown bool) int {
	if local.Main == drive.Main {
		return DSyncAlready
	}
	if local.Main.After(drive.Main) {
		if acceptUp {
			return DSyncUp
		}
		return DSyncBlockedUP
	}
	if acceptDown {
		return DSyncDown
	}
	return DSyncBlockedDown
}

func DiffRepos(from DRepoDate, to DRepoDate) DRepoDiff {
	r := DRepoDiff{
		Copy:   make([]string, 0),
		Delete: make([]string, 0),
	}
	for k := range from.Files {
		_, exist := to.Files[k]
		if !exist {
			r.Delete = append(r.Delete, k)
		}
	}
	for k := range to.Files {
		tv, _ := to.Files[k]
		fv, exists := from.Files[k]
		if !exists || tv.UnixMilli() != fv.UnixMilli() {
			r.Copy = append(r.Copy, k)
		}
	}
	return r
}
