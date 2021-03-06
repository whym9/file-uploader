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
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	zmq "github.com/pebbe/zmq4"
)

var maxSize int64
var uploadPath string
var url string

func recordMetrics() {
	go func() {
		for {
			opsProcessed.Inc()
			time.Sleep(2 * time.Second)
		}
	}()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
)

func main() {
	url = *flag.String("url", "localhost:8080", "web site url")
	uploadPath = *flag.String("path", "./files", "file upload path")
	maxSize = *flag.Int64("maxSize", 2*1024*1024, "maximum size of the file")

	err := os.MkdirAll(uploadPath, os.ModePerm)
	if err != nil {
		log.Fatalf("couldn't create path, %v", err)
	}
	recordMetrics()
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", uploadFile)
	http.HandleFunc("/results", results)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Printf("Server started on %v\n", url)
	log.Fatal(http.ListenAndServe(url, nil))

}

var wg sync.WaitGroup

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
	if fileType != "application/octet-stream" {
		t, err := template.ParseFiles("static/upload.gohtml")
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}
		mes := struct{ Message string }{Message: "Wrong file type!"}
		err = t.Execute(w, mes)
		if err != nil {
			http.ServeFile(w, r, "static/error.html")

		}
		return
	}

	fileName := fileHeader.Filename

	newPath := filepath.Join(uploadPath, fileName)
	fmt.Printf("FileType: %s, File: %s\n", fileType, newPath)
	fmt.Printf("File size (bytes): %v\n", fileSize)

	err = saveFile(fileContent, newPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("CANT_SAVE_FILE"))
		log.Fatal(err)
		return
	} else {
		// t, err := template.ParseFiles("static/upload.gohtml")
		// if err != nil {
		// 	http.ServeFile(w, r, "static/error.html")
		// }
		// mes := struct{ Message string }{Message: "File " + fileName + " was successfully added!\n"}
		// err = t.Execute(w, mes)
		// if err != nil {
		// 	http.ServeFile(w, r, "static/error.html")

		// }
		countTCPAndUDP(newPath)
		http.Redirect(w, r, "/results", http.StatusSeeOther)
		wg.Add(1)
		go zeroMQSend(newPath)
		wg.Wait()
		return
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

var (
	eth layers.Ethernet
	ip4 layers.IPv4
	ip6 layers.IPv6
	tcp layers.TCP
	udp layers.UDP
	dns layers.DNS
)

type Protocols struct {
	TCP  int
	UDP  int
	IPv4 int
	IPv6 int
}

var counter Protocols

func countTCPAndUDP(file string) {
	parser := gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&eth,
		&ip4,
		&ip6,
		&tcp,
		&udp,
		&dns,
	)

	handle, err := pcap.OpenOffline(file)

	if err != nil {
		log.Fatal(err)
	}

	defer handle.Close()

	if err != nil {
		log.Fatal(err)
	}
	decoded := make([]gopacket.LayerType, 0, 10)
	for {
		data, _, err := handle.ZeroCopyReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		parser.DecodeLayers(data, &decoded)

		for _, layer := range decoded {
			if layer == layers.LayerTypeTCP {
				counter.TCP++
			}
			if layer == layers.LayerTypeUDP {
				counter.UDP++
			}
			if layer == layers.LayerTypeIPv4 {
				counter.IPv4++
			}
			if layer == layers.LayerTypeIPv6 {
				counter.IPv6++
			}
		}
	}

}
func results(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, err := template.ParseFiles("static/view.gohtml")
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}

		err = t.Execute(w, counter)
		if err != nil {
			http.ServeFile(w, r, "static/error.html")
		}

		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func print(arg map[string]int) {
	fmt.Println("Amounts of TCP and UDP:")
	for protocol, amount := range arg {
		fmt.Printf("%v: %v\n", protocol, amount)
	}
}

func zeroMQSend(name string) {
	handle, err := pcap.OpenOffline(name)

	if err != nil {
		panic(err)
	}
	defer handle.Close()

	x := zmq.PUB
	socket, err := zmq.NewSocket(x)
	if err != nil {
		log.Fatal()
	}

	defer socket.Close()
	socket.Bind("tcp://*:5556")

	fmt.Println("Sending messages on port 5556")

	for {
		data, _, err := handle.ZeroCopyReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		socket.SendBytes(data, 0)
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println("Stopped sending file")
	wg.Done()
}
