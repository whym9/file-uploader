package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"

	"flag"
	"net/http"
	"os"
	"path/filepath"
)

var maxSize int64
var uploadPath string
var url string

func main() {
	url = *flag.String("url", "localhost:8080", "web site url")
	uploadPath = *flag.String("path", "./files", "file upload path")
	maxSize = *flag.Int64("maxSize", 2*1024*1024, "maximum size of the file")
	http.HandleFunc("/", uploadFile)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Printf("Server started on %v\n", url)
	log.Fatal(http.ListenAndServe(url, nil))
}

func uploadFile(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		t, _ := template.ParseFiles("static/upload.gohtml")

		t.Execute(w, nil)
		return
	}

	if err := r.ParseMultipartForm(maxSize); err != nil {
		fmt.Printf("Could not parse multipart form: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("CANT_PARSE_FORM"))
		return
	}

	file, fileHeader, err := r.FormFile("uploadFile")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("INVALID_FILE"))
		return
	}
	defer file.Close()

	fileSize := fileHeader.Size

	if fileSize > maxSize {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("FILE_TOO_BIG"))
		return
	}
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("INVALID_FILE"))
		return
	}

	fileType := http.DetectContentType(fileContent)

	fileName := fileHeader.Filename

	newPath := filepath.Join(uploadPath, fileName)
	fmt.Printf("FileType: %s, File: %s\n", fileType, newPath)
	fmt.Printf("File size (bytes): %v\n", fileSize)

	// write file

	result := saveFile(fileContent, newPath)
	if !result {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("CANT_SAVE_FILE"))
		return
	} else {
		fmt.Fprintln(w, "File was successfully uploaded!")
		fmt.Fprintf(w, "FileType: %s, File: %s\n", fileType, newPath)
		fmt.Fprintf(w, "File size (bytes): %v\n", fileSize)
	}
}

func saveFile(content []byte, path string) bool {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println("Counldn't create file")
		return false
	}

	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		fmt.Println("Counldn't write to file")
		return false
	}
	return true
}
