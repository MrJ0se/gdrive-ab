package main

import (
	"encoding/json"
	"fmt"
	"gdriveAB/driveFS"
	"gdriveAB/driveSync"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	//"os"
)

type ConfigFolder struct {
	Local string `json:"local"`
	Drive string `json:"drive"`
}
type Config struct {
	Folders []ConfigFolder `json:"folders"`
}

func syncConfigFolder(dfs *driveFS.DFS, cf ConfigFolder, down bool) {
	local_t := driveSync.NewDRepoDate_local(cf.Local, nil)
	drive_t := driveSync.NewDRepoDate_drive(dfs, cf.Drive, nil)

	if down {
		diff := driveSync.DiffRepos(local_t, drive_t)
		driveSync.SyncDown(dfs, cf.Drive, cf.Local, diff, true)
	} else {
		diff := driveSync.DiffRepos(drive_t, local_t)
		driveSync.SyncUp(dfs, cf.Drive, cf.Local, diff, true)
	}
}

func main() {
	alen := len(os.Args)

	if alen < 2 {
		fmt.Print(
			`Usage: configFile [upsync/downsync]
Need a configFile.json.
If configFile.token.json file not exists, you ill be required to do login in a new gmail.`)
		return
	}

	commandFile := strings.ToLower(os.Args[1])

	//readfile
	jsonFile, err := os.Open(commandFile + ".json")
	if err != nil {
		panic(err)
	}
	byteValue, _ := ioutil.ReadAll(jsonFile)
	jsonFile.Close()
	//unmarshal
	var config Config
	json.Unmarshal(byteValue, &config)

	d := driveFS.NewDFS(commandFile + ".token.json")
	d.DriveEPanic = true

	if alen < 3 {
		return
	}
	commandOp := strings.ToLower(os.Args[2])
	if commandOp == "upsync" {
		flen := strconv.Itoa(len(config.Folders))
		for i, f := range config.Folders {
			fmt.Println("[UpSync " + strconv.Itoa(i) + "/" + flen + "] " + f.Local)
			syncConfigFolder(d, f, false)
		}
	} else if commandFile == "downsync" {
		flen := strconv.Itoa(len(config.Folders))
		for i, f := range config.Folders {
			fmt.Println("[DownSync " + strconv.Itoa(i) + "/" + flen + "] " + f.Local)
			syncConfigFolder(d, f, true)
		}
	} else {
		panic("unknow: " + commandOp)
	}

	/*dir, _ := os.Getwd()

	local_t := driveSync.NewDRepoDate_local(dir)
	drive_t := driveSync.NewDRepoDate_drive(d, "synctest")
	diff := driveSync.DiffRepos(drive_t, local_t)
	driveSync.SyncUp(d, "synctest", dir, diff, true)*/

	/*d := driveFS.NewDFS("token.json")
	d.DriveEPanic = true

	f, err := d.File("deno-cct.csv")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v (%v, size=%v)\n", f.Path, f.Id, f.Size)
	fmt.Print(f)

	f.GarantPropertiesMap()
	f.Properties["gdrive-api-teste"] = "key"
	d.Update(f)
	//*/

	/*x := DriveFS.DFile{
		Path:     "testFile",
		IsFolder: false,
	}
	y, err := d.Create(x)
	if err != nil {
		panic(err)
	}
	if y == nil {
		panic("nil return file")
	}
	fmt.Print(y)
	fr, _ := os.Open("rules.go")
	d.Upload(y, fr)
	//*/
	/*l, err := d.ReadDir("gab")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Files (%v):\n", len(l))
	for _, e := range l {
		fmt.Printf("%v (Folder: %v, size: %v)\n", e.Name, e.IsFolder, e.Size)
	} //*/
}
