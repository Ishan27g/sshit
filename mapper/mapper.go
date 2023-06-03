package mapper

import (
	"fmt"
	"io"
	"time"
)

var tunnelId = 0

type HttpTunnel struct {
	HttpW io.Writer
	done  chan bool
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

func (m *Mapper) HttpReady(id int, w io.Writer) chan bool {
	if m.tunnels[id] == nil {
		return nil
	}
	done := make(chan bool)
	m.tunnels[id] <- HttpTunnel{HttpW: w, done: done}
	return done
}

func (m *Mapper) SshIt(id int, r io.Reader) *Report {
	if m.tunnels[id] == nil {
		return nil
	}
	rp := &Report{}
	waitStart := time.Now()
	// blocked until sent by Ready
	ht := <-m.tunnels[id]
	rp.Wait = time.Since(waitStart).String()
	copyStart := time.Now()
	fmt.Println("received from channel")
	pr, pw := io.Pipe()
	intercepted := make(chan bool)
	go func() {
		all, err := io.ReadAll(pr)
		if err != nil {
			return
		}
		rp.Intercepted = all
		intercepted <- true
	}()
	mr := io.MultiWriter(ht.HttpW, pw)
	io.Copy(mr, r)
	pw.Close()
	rp.Copy = time.Since(copyStart).String()
	close(ht.done)
	m.Clean(id)
	<-intercepted
	return rp
}
func (m *Mapper) Clean(id int) {
	delete(m.tunnels, id)
}