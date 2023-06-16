package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ishan27g/sshit/cli"
	"github.com/Ishan27g/sshit/data"
	"github.com/Ishan27g/sshit/mapper"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/savioxavier/termlink"
)

const MaxUploadSize = 104800 * 1024 // 100MB
var host = ""

func init() {
	host, _ = os.Hostname()
	if strings.Contains(host, "local") {
		host = "http://localhost:8080"
	} else {
		host = "https://sshit.onrender.com"
	}
}

//var init = flag.Bool("d", false, "create download link with file contents. (default behaviour will create download link for file)")

type sshit struct {
	tunnels *mapper.Mapper
}

func main() {
	var asData = flag.Bool("d", false, "create download link with file contents. (default behaviour will create download link for file)")

	if len(os.Args) >= 2 {
		flag.Parse()
		link, id := cli.ReqUpload()
		l := fmt.Sprintf("%s/%d", link, id)
		l = termlink.ColorLink("link", l, "green", true)
		l = strings.ReplaceAll(l, "(", "")
		l = strings.ReplaceAll(l, ")", "")
		fmt.Println(fmt.Sprintf("\n\n\t%s\n\n", l))

		buf := bufio.NewReader(os.Stdin)
		if *asData {
			fmt.Print("> Start Upload? ⏎")
		} else {
			fmt.Print("> Start Upload? ⏎")
		}
		sentence, err := buf.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(sentence))
		}
		if *asData {
			cli.UploadFileAsBinary(id, os.Args[2])
		} else {
			cli.UploadFileAsFormData(id, os.Args[1])
		}
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
	go func() {
		for {
			// keepalive for render
			_, _ = http.Get(host + "/keepalive")
			<-time.After(1 * time.Minute)
		}
	}()
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
	m.HandleFunc("/keepalive", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("alive"))
		fmt.Println("keepalive")
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
			fmt.Println("setting filename", f, "[]")
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
			file, _, err := request.FormFile("file")
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			fname := request.FormValue("name")
			r := sht.tunnels.SshIt(id, file, fname)
			defer sht.tunnels.Clean(id)
			//fmt.Println(string(r.Intercepted))
			fmt.Println(r.Wait, r.Copy)
		} else {
			r := sht.tunnels.SshIt(id, request.Body, "")
			defer sht.tunnels.Clean(id)
			//fmt.Println(string(r.Intercepted))
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
		//	fmt.Println(string(r.Intercepted))
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
