package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ishan27g/sshit/data"
)

var host = os.Getenv("HOST")

func init() {
	//if host == "" {
	//	host = "http://localhost:8080"
	//} else {
	//}
	host = "https://sshit.onrender.com"
}

func defaultHttpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: 15 * time.Minute,
		},
		Timeout: 15 * time.Minute,
	}
}
func ReqUpload() (string, int) {
	return requestUpload()
}
func requestUpload() (string, int) {
	req, _ := http.NewRequest("GET", host+"/upload", nil)
	req.Header.Set("User-Agent", "IE=Edge")

	resp, err := defaultHttpClient().Do(req)
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	var urlRsp = &data.UrlResponse{}
	err = json.Unmarshal(body, urlRsp)
	if err == nil {
		return urlRsp.DDlink, urlRsp.Id
	}
	fmt.Println(string(body))
	return "", -1
}
func UploadFileAsBinary(id int, fileName string) string {
	b, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/upload/%d", host, id), b)
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	req.Header.Set("User-Agent", "IE=Edge")

	resp, err := defaultHttpClient().Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return ""
}
func UploadFileAsFormData(id int, fileName string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	values := map[string]io.Reader{
		"file": f,
		"name": strings.NewReader(filepath.Base(fileName)),
	}
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if x, ok := r.(*os.File); ok {
			if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
				return
			}
		} else {
			if fw, err = w.CreateFormField(key); err != nil {
				return
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			log.Fatal(err)
		}

	}
	w.Close()
	url := fmt.Sprintf("%s/upload/%d", host, id)

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return
	}
	req.Header.Set("SSHIT_FILE", fileName)
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := defaultHttpClient().Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	bb, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(bb))
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	return
}
