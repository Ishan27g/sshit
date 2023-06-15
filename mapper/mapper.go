package mapper

import (
	"fmt"
	"io"
	"time"
)

var tunnelId = 0

type filename string
type HttpTunnel struct {
	HttpW    io.Writer
	fileName chan filename
	done     chan bool
}
type Mapper struct {
	tunnels map[int]chan HttpTunnel
}

type SshTunnel struct {
	id   int
	Done func()
}

type Report struct {
	Wait        string
	Copy        string
	Intercepted []byte
}

func Init() *Mapper {
	return &Mapper{tunnels: map[int]chan HttpTunnel{}}
}
func (m *Mapper) Create() int {
	tunnelId++
	m.tunnels[tunnelId] = make(chan HttpTunnel)
	return tunnelId
}

func (m *Mapper) HttpReady(id int, w io.Writer) (chan filename, chan bool) {
	if m.tunnels[id] == nil {
		return nil, nil
	}
	fileName := make(chan filename)
	done := make(chan bool)
	m.tunnels[id] <- HttpTunnel{HttpW: w, fileName: fileName, done: done}
	return fileName, done
}

func (m *Mapper) SshIt(id int, r io.Reader, fileName string) *Report {
	if m.tunnels[id] == nil {
		return nil
	}
	rp := &Report{}
	waitStart := time.Now()
	// blocked until sent by Ready
	ht := <-m.tunnels[id]
	ht.fileName <- filename(fileName)
	<-ht.fileName
	close(ht.fileName)
	rp.Wait = time.Since(waitStart).String()
	copyStart := time.Now()
	fmt.Println("received from channel")
	{
		_, _ = io.Copy(ht.HttpW, r)
	}
	//pr, pw := io.Pipe()
	//intercepted := make(chan bool)
	//go func() {
	//	all, err := io.ReadAll(pr)
	//	if err != nil {
	//		return
	//	}
	//	rp.Intercepted = all
	//	intercepted <- true
	//}()
	//mr := io.MultiWriter(ht.HttpW, pw)
	//io.Copy(mr, r)
	//pw.Close()
	rp.Copy = time.Since(copyStart).String()
	close(ht.done)
	m.Clean(id)
	//<-intercepted
	return rp
}
func (m *Mapper) Clean(id int) {
	delete(m.tunnels, id)
}
