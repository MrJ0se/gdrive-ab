package driveSync

import (
	"fmt"
	"gdriveAB/driveFS"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

func SyncUp(dfs *driveFS.DFS, driveRoot string, localRoot string, diff DRepoDiff, doDeletes bool) {
	localRoot = path.Join(localRoot)
	fmt.Println("running SyncUp")
	//copy
	tcpy := len(diff.Copy)
	for i, k := range diff.Copy {
		fmt.Println("[Cpy " + strconv.Itoa(i) + "/" + strconv.Itoa(tcpy) + "]" + k)
		dff, err := dfs.CreateRecursive(driveFS.DFile{
			Name:       "",
			Path:       strings.ReplaceAll(driveRoot+"/"+k, "//", "/"),
			Id:         "",
			TimeMod:    time.UnixMilli(0),
			Properties: nil,
			IsFolder:   false,
			Size:       0,
		})
		if err != nil {
			panic(err)
		}
		lpath := path.Join(localRoot, k)
		err = dfs.UploadFile(dff, lpath)
		if err != nil {
			panic(err)
		}
		//update time footprint
		lff, err := os.Stat(lpath)
		if err != nil {
			panic(err)
		}
		dff.GarantPropertiesMap()
		dff.Properties["dab-time"] = strconv.FormatInt(lff.ModTime().UTC().UnixMilli(), 10)
		err = dfs.Update(dff)
		if err != nil {
			panic(err)
		}
	}

	//deletes
	if doDeletes {
		tdel := len(diff.Delete)
		for i, k := range diff.Delete {
			fmt.Println("[Del " + strconv.Itoa(i) + "/" + strconv.Itoa(tdel) + "]" + k)
			current := strings.ReplaceAll(driveRoot+"/"+k, "//", "/")
			f, err := dfs.File(current)
			if err != nil {
				fmt.Println("Not found, cant delete")
				continue
			}
			err = dfs.DeleteID(f.Id)
			if err != nil {
				fmt.Println("Delete failed or connection lost, stopping delete sequence")
				break
			}
		}
	}
	//update time footprint
	lff, err := os.Stat(localRoot)
	if err != nil {
		panic(err)
	}
	dff, err := dfs.File(driveRoot)
	if err != nil {
		panic(err)
	}
	dff.GarantPropertiesMap()
	dff.Properties["dab-time"] = strconv.FormatInt(lff.ModTime().UTC().UnixMilli(), 10)
	err = dfs.Update(dff)
	if err != nil {
		panic(err)
	}
}
func mkdirAllIN(folderPath string) {
	s, e := os.Stat(folderPath)
	if e == nil {
		if !s.IsDir() {
			os.Remove(folderPath)
		}
		return
	}
	e = os.MkdirAll(folderPath, os.ModePerm)
}
func SyncDown(dfs *driveFS.DFS, driveRoot string, localRoot string, diff DRepoDiff, doDeletes bool) {
	last_mkdir := ""
	localRoot = path.Join(localRoot)
	fmt.Println("running SyncDown")
	//copy
	tcpy := len(diff.Copy)
	for i, k := range diff.Copy {
		fmt.Println("[Cpy " + strconv.Itoa(i) + "/" + strconv.Itoa(tcpy) + "]" + k)

		folderPath := path.Join(localRoot, k, "..")
		if folderPath != last_mkdir {
			if strings.HasPrefix(folderPath, last_mkdir) {
				mkdirAllIN(folderPath)
			}
			last_mkdir = folderPath
		}

		dff, err := dfs.File(driveRoot + "/" + k)
		if err != nil {
			panic(err)
		}
		lpath := path.Join(localRoot, k)
		err = dfs.DownloadFile(dff, lpath)
		if err != nil {
			panic(err)
		}
	}

	//deletes
	if doDeletes {
		tdel := len(diff.Delete)
		for i, k := range diff.Delete {
			fmt.Println("[Del " + strconv.Itoa(i) + "/" + strconv.Itoa(tdel) + "]" + k)
			current := path.Join(localRoot, k)
			err := os.Remove(current)
			if err != nil {
				fmt.Println("Delete failed: " + err.Error())
			}
		}
	}
}
