package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Ishan27g/sshit/cli"
	"github.com/Ishan27g/sshit/data"
	"github.com/Ishan27g/sshit/mapper"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

const MaxUploadSize = 10240 * 1024 // 10MB
var host = os.Getenv("HOST")

type sshit struct {
	tunnels *mapper.Mapper
}

func main() {

	if len(os.Args) == 2 {
		link, id := cli.ReqUpload()
		fmt.Println(fmt.Sprintf("%s%d", link, id))
		buf := bufio.NewReader(os.Stdin)
		fmt.Print("> Start UploadFileAsBinary? ⏎")
		sentence, err := buf.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(sentence))
		}
		cli.UploadFileAsBinary(id, os.Args[1])
		return
	}

	s := &sshit{}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	//sshPort := os.Getenv("SSH_PORT")
	//if sshPort == "" {
	//	sshPort = ":10022"
	//}
	s.tunnels = mapper.Init()
	s.httpServer(":" + port)

	//s.sshInit(sshPort)
}
func (sht *sshit) httpServer(port string) {
	m := mux.NewRouter()
	cr := cors.New(cors.Options{
		//AllowedOrigins:   []string{"http://127.0.0.1:5173"},
		//AllowedMethods:   []string{http.MethodGet},
		//AllowedHeaders:   []string{"*"},
		//AllowCredentials: false,
	})
	m.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("ok"))
		return
	})
	//	m.Handle("/mapView/", http.StripPrefix("/mapView/", http.FileServer(http.Dir("./mapView"))))
	m.HandleFunc("/download/{id}", func(writer http.ResponseWriter, request *http.Request) {
		idStr := mux.Vars(request)["id"]
		id, _ := strconv.Atoi(idStr)
		fmt.Println("started download for ", id)

		filename, done := sht.tunnels.HttpReady(id, writer)
		if filename == nil || done == nil {
			writeErr(writer, `{"error":"id not found"}`)
			return
		}
		f := <-filename
		if f != "" {
			writer.Header().Set("Content-Disposition", "attachment; filename="+string(f))
			writer.Header().Set("Content-Type", request.Header.Get("Content-Type"))
		} else {
			fmt.Println("not setting filename")
		}
		filename <- ""
		<-done
		return
	})
	m.HandleFunc("/upload", func(writer http.ResponseWriter, request *http.Request) {
		id := sht.tunnels.Create()
		if host == "" {
			host = "http://localhost:8080"
		} else {
			host = "https://sshit.onrender.com"
		}
		b, _ := json.Marshal(&data.UrlResponse{DDlink: host + "/download", Id: id})
		writer.WriteHeader(http.StatusCreated)
		writer.Write(b)
		fmt.Println("initialised upload for ", id)
	})
	m.HandleFunc("/upload/{id}", func(writer http.ResponseWriter, request *http.Request) {
		//data := mux.Vars(request)["data"]
		//id := sht.tunnels.Create()
		//r := sht.tunnels.SshIt(id, strings.NewReader(data))
		idStr := mux.Vars(request)["id"]
		id, _ := strconv.Atoi(idStr)
		fmt.Println("requested upload for ", id)
		if request.Header.Get("SSHIT_FILE") != "" {
			request.Body = http.MaxBytesReader(writer, request.Body, MaxUploadSize)
			_ = request.ParseMultipartForm(MaxUploadSize)
			file, fileHeader, err := request.FormFile("file")
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			fname := request.FormValue("name")
			fmt.Println(*fileHeader)
			r := sht.tunnels.SshIt(id, file, fname)
			defer sht.tunnels.Clean(id)
			fmt.Println(string(r.Intercepted))
			fmt.Println(r.Wait, r.Copy)
		} else {
			r := sht.tunnels.SshIt(id, request.Body, "")
			defer sht.tunnels.Clean(id)
			fmt.Println(string(r.Intercepted))
			fmt.Println(r.Wait, r.Copy)
		}

	})
	handler := cr.Handler(m)

	fmt.Println("starting http on port", port)
	http.ListenAndServe(port, handler)
}
func writeErr(writer http.ResponseWriter, data interface{}) {
	b, _ := json.Marshal(data)
	writer.WriteHeader(http.StatusBadRequest)
	writer.Write(b)
}

var (
	DeadlineTimeout = 30 * time.Second
	IdleTimeout     = 10 * time.Second
)

func (sht *sshit) sshInit(port string) {
	ssh.Handle(func(s ssh.Session) {
		id := sht.tunnels.Create()
		s.Write([]byte(fmt.Sprintf("http://localhost:8090/download/%d\n", id)))
		r := sht.tunnels.SshIt(id, s, "")
		defer sht.tunnels.Clean(id)
		fmt.Println(string(r.Intercepted))
		fmt.Println(r.Wait, r.Copy)
		s.Write([]byte("done"))
	})

	fmt.Println("starting ssh server on port", port)
	server := &ssh.Server{
		Addr:        port,
		MaxTimeout:  DeadlineTimeout,
		IdleTimeout: IdleTimeout,
	}
	fmt.Println(server.ListenAndServe())
}
