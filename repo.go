package main

import (
	"fmt"
	"io/fs"
)


func GetFile(){

	FS := fs.FileMode.String(fs.ModeDir)
	fmt.Println(FS)
}
