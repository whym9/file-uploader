package main

import (
	"fmt"
	"html/template"
	"io"
	"log"

	"errors"
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

	err := os.MkdirAll(uploadPath, os.ModePerm)
	if err != nil {
		log.Printf("couldn't create path, %v", err)
	}
	http.HandleFunc("/", uploadFile)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Printf("Server started on %v\n", url)
	log.Fatal(http.ListenAndServe(url, nil))
}

func uploadFile(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		t, err := template.ParseFiles("static/upload.gohtml")
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}

		err = t.Execute(w, nil)
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}

		return
	}

	if err := r.ParseMultipartForm(maxSize); err != nil {
		fmt.Printf("could not parse multipart form: %v\n", err)
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
	fileContent, err := io.ReadAll(file)
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

	err = saveFile(fileContent, newPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("CANT_SAVE_FILE"))
		log.Fatal(err)
		return
	} else {
		t, err := template.ParseFiles("static/upload.gohtml")
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}
		mes := struct{ Message string }{Message: "File  was successfully added!\n"}
		err = t.Execute(w, mes)
		if err != nil {
			http.ServeFile(w, r, "static/error.html")

		}
	}
}

func saveFile(content []byte, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.New("couldn't create file")
	}

	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return errors.New("counldn't write to file")
	}
	return nil
}
