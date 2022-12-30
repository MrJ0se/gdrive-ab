package driveFS

import (
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
)

type DFS struct {
	Srv         *drive.Service
	DriveEPanic bool
	pcache      map[string]string //folders token cache
}

func NewDFS(tkFile string) *DFS {
	srv := SetupDriveService(tkFile)
	if srv == nil {
		return nil
	}
	return &DFS{srv, true, make(map[string]string)}
}

// return (ID, isfolder, exists, DriveError)
func (x *DFS) discoveryID(frags []string) (string, bool, bool, error) {
	flen := len(frags)
	if flen == 0 {
		return "root", true, true, nil
	}
	joined := strings.Join(frags, "/")

	tk, vld := x.pcache[joined]
	if vld {
		return tk, true, true, nil
	}

	parent, isFolder, exists, err := x.discoveryID(frags[:flen-1])
	if !exists || !isFolder {
		return "", false, false, err
	}
	rd, err := x.Srv.Files.List().
		Q("'" + parent + "' in parents and name = '" + frags[flen-1] + "' and trashed=false").
		PageSize(1).
		Fields("files(id, mimeType)").Do()
	if err != nil || len(rd.Files) == 0 {
		return "", false, false, err
	}

	fid := rd.Files[0].Id
	isFolder = (rd.Files[0].MimeType == "application/vnd.google-apps.folder")
	if isFolder {
		x.pcache[joined] = fid
	}

	return fid, isFolder, true, nil
}
func (x *DFS) metaFile(id string) (*drive.File, error) {
	rd, err := x.Srv.Files.Get(id).
		Fields("id, name, mimeType, size, modifiedTime, properties, sha1Checksum").
		Do()
	if err != nil {
		return nil, err
	}
	if rd != nil && rd.Name != "" {
		return rd, nil
	}
	return nil, nil
}
func (x *DFS) enumFilesTk(tk string, addq string) ([]*drive.File, error) {
	res := []*drive.File{}
	npt := ""
	for true {
		query := x.Srv.Files.List().
			Q("'" + tk + "' in parents" + addq + " and trashed=false").
			PageSize(100).
			Fields("files(id, name, mimeType, size, modifiedTime, properties, sha1Checksum)")
		if npt != "" {
			query = query.PageToken(npt)
		}
		rd, err := query.Do()
		if err != nil {
			return res, err
		}
		res = append(res, rd.Files...)

		if rd.NextPageToken == "" || len(rd.Files) < 100 {
			break
		}
	}
	return res, nil
}

func driveToDFile(ed *drive.File, folderpath []string) *DFile {
	dtime, _ := time.Parse(time.RFC3339, ed.ModifiedTime)
	return &DFile{
		Id:           ed.Id,
		Name:         ed.Name,
		Path:         strings.Join(append(folderpath, ed.Name), "/"),
		TimeMod:      dtime,
		Properties:   ed.Properties,
		IsFolder:     ed.MimeType == "application/vnd.google-apps.folder",
		Size:         ed.Size,
		Sha1Checksum: ed.Sha1Checksum,
	}
}

func (x *DFS) ReadDir(name string) ([]DFile, error) {
	res := []DFile{}

	np := newApath(name)

	dId, dFolder, dExists, err := x.discoveryID(np.names)
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return res, err
	}
	if !dExists {
		return res, fs.ErrNotExist
	}
	if !dFolder {
		return res, fs.ErrInvalid
	}
	dRes, err := x.enumFilesTk(dId, "")
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return res, err
	}
	for _, ed := range dRes {
		res = append(res, *driveToDFile(ed, np.names))
	}
	return res, nil
}
func (x *DFS) File(name string) (*DFile, error) {
	np := newApath(name)
	npl := len(np.names)
	dId, _, dExists, err := x.discoveryID(np.names)
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return nil, err
	}
	if !dExists {
		return nil, fs.ErrNotExist
	}
	dRes, err := x.metaFile(dId)
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return nil, err
	}
	if dRes != nil {
		return driveToDFile(dRes, np.names[:npl-1]), nil
	}
	return nil, fs.ErrNotExist
}
func (x *DFS) CreateRecursive(file DFile) (*DFile, error) {
	kp := newApath(file.Path)
	k, kl := kp.names, len(kp.names)-1

	for i := 0; i < kl; i++ {
		fpath := k[:i+1]
		_, _, dExists, err := x.discoveryID(fpath)
		if err != nil {
			return nil, err
		}
		if dExists {
			continue
		}
		_, err = x.Create(DFile{
			Id:         "",
			Name:       k[i],
			Path:       strings.Join(fpath, "/"),
			TimeMod:    time.UnixMilli(0),
			Properties: nil,
			IsFolder:   true,
			Size:       0,
		})
		if err != nil {
			return nil, err
		}
	}

	return x.Create(file)
}
func (x *DFS) Create(file DFile) (*DFile, error) {
	kp := newApath(file.Path)
	k, kl := kp.names, len(kp.names)
	file.Path = kp.npath
	if file.Name == "" {
		if kl == 0 {
			return nil, fs.ErrInvalid
		}
		file.Name = k[kl-1]
	} else if file.Path == "" {
		file.Path = file.Name
		k = []string{file.Name}
	}

	xFile, err := x.File(file.Path)
	if err == nil {
		if xFile != nil {
			if xFile.IsFolder != file.IsFolder {
				return nil, fs.ErrExist
			}
			return xFile, nil
		}
	} else if err != fs.ErrNotExist {
		return nil, err
	}

	dId, dIsFolder, dExists, err := x.discoveryID(k[:kl-1])
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return nil, err
	}
	if !dExists {
		return nil, fs.ErrNotExist
	}
	if !dIsFolder {
		return nil, fs.ErrInvalid
	}

	mime := ""
	if file.IsFolder {
		mime = "application/vnd.google-apps.folder"
	}
	res, err := x.Srv.Files.Create(&drive.File{
		Parents:    []string{dId},
		Properties: file.Properties,
		Name:       file.Name,
		MimeType:   mime,
	}).Do()

	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return nil, err
	}
	if res == nil {
		return nil, nil
	}

	if kl > 0 {
		return driveToDFile(res, k[:kl-1]), nil
	}
	return driveToDFile(res, k), nil
}
func (x *DFS) DeleteID(dId string) error {
	err := x.Srv.Files.Delete(dId).Do()
	if err != nil && x.DriveEPanic {
		panic(err)
	}
	return err
}
func (x *DFS) Delete(name string) error {
	np := newApath(name)
	dId, _, dExists, err := x.discoveryID(np.names)
	if err != nil {
		if x.DriveEPanic {
			panic(err)
		}
		return err
	}
	if !dExists {
		return fs.ErrNotExist
	}
	return x.DeleteID(dId)
}

// update properties only
func (x *DFS) Update(file *DFile) error {
	_, err := x.Srv.Files.Update(file.Id, &drive.File{
		Properties: file.Properties,
	}).Do()

	if err != nil && x.DriveEPanic {
		panic(err)
	}
	return err
}
func (x *DFS) Upload(file *DFile, reader io.Reader) error {
	_, err := x.Srv.Files.Update(file.Id, nil).Media(reader).Do()
	if err != nil && x.DriveEPanic {
		panic(err)
	}
	return err
}
func (x *DFS) UploadFile(file *DFile, local string) error {
	meta, err := NewUtilHashSize(local)
	if err != nil || (file.Size > 0 && meta.Size == file.Size && meta.Sha1 == file.Sha1Checksum) {
		return err
	}

	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	return x.Upload(file, f)
}

func (x *DFS) Download(file *DFile) (io.ReadCloser, error) {
	res, err := x.Srv.Files.Get(file.Id).Download()
	if err != nil && x.DriveEPanic {
		panic(err)
	}
	if res == nil || err != nil {
		return nil, err
	}
	return res.Body, err
}
func (x *DFS) DownloadFile(file *DFile, local string) error {
	meta, err := NewUtilHashSize(local)

	if err == nil && meta.Size == file.Size && meta.Sha1 == file.Sha1Checksum {
		return nil
	}

	res, err := x.Download(file)
	if err != nil {
		return err
	}
	defer res.Close()
	out, err := os.Create(local)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, res)
	return err
}
