package main

import (
	"os"
	"strconv"
	"golang.org/x/crypto/ssh/terminal"
	"time"
	"math/rand"
	"io/ioutil"
	"sync"
	"strings"
)

type ClientData struct {
    filepath string
    count chan int
}

func NewClientData() *ClientData {
	p := new(ClientData)
	p.filepath = ""
	p.count = make(chan int)
	return p
}

var term *terminal.Terminal

func TermPrintnl(s string){
	term.Write([]byte(s + "\n"))
}

var succMutex sync.Mutex = sync.Mutex{}
var success int = 0
var success5s int = 0

var failMutex sync.Mutex = sync.Mutex{}
var failed int = 0
var failed5s int = 0

var activeWorkersMutex sync.Mutex = sync.Mutex{}
var activeWorkers int = 0

var clientCount int = 10
var workerCount int = 2

var clientIntensity int = 50
var workerWait int = 20

func worker(cnl chan *ClientData, stopChan chan bool){
	for {
		data := <-cnl
		
		activeWorkersMutex.Lock()
		activeWorkers += 1
		activeWorkersMutex.Unlock()
		
		b, err := ioutil.ReadFile(data.filepath)
		if err != nil{
			panic(err)
		}
		str := string(b)
		word_arr := strings.Fields(str)
		data.count <- len(word_arr)
	
		newTaskTimer := time.After(time.Millisecond * time.Duration(workerWait))
		select{
			case <-stopChan:
				return
			case <-newTaskTimer:
				//nothing, continue iteration
		}
	}
	
}

func client(cnl chan *ClientData, stopChan chan bool){
	for {
		
		data := NewClientData()
		data.filepath = os.Getenv("GOPATH")
		data.filepath += "/src/github/dist_go_lab/20_newsgroup"
		files, err := ioutil.ReadDir(data.filepath)
		if err != nil{
			panic(err)
		}
		data.filepath += "/" + files[rand.Intn(len(files))].Name()
		files, err = ioutil.ReadDir(data.filepath)
		if err != nil{
			panic(err)
		}
		data.filepath += "/" + files[rand.Intn(len(files))].Name()
		//TermPrintnl(data.filepath)
	
		cnl <- data
	
		wait := time.After(time.Millisecond * 100)
		select{
			case <-wait:
				failMutex.Lock()
				failed += 1
				failed5s += 1
				failMutex.Unlock()
			case <-data.count:
				succMutex.Lock()
				success += 1
				success5s += 1
				succMutex.Unlock()
		}
		
		newTaskTimer := time.After(time.Millisecond * time.Duration(clientIntensity))
		
		select{
			case <-stopChan:
				return
			case <-newTaskTimer:
				//nothing, continue iteration
		}
	}
	
}

func control(qiut chan bool, keyEvent chan rune){
	dataChanel := make(chan *ClientData)
	stopClientChan := make(chan bool)
	stopWorkerChan := make(chan bool)
	
	for i := 0; i < workerCount; i++ {
		go worker(dataChanel, stopWorkerChan)
	}
	
	for i := 0; i < clientCount; i++ {
		go client(dataChanel, stopClientChan)
	}
	
	infoTimer := time.Tick(time.Second * 5)
	statTimer := time.Tick(time.Second * 1)
	for {
		//TermPrintnl("Iteration")
		select{
			case <-infoTimer:
				failMutex.Lock()
				succMutex.Lock()
				
				TermPrintnl("----------------------------")
				TermPrintnl("Clients: " + strconv.Itoa(clientCount) + ", workers: " + strconv.Itoa(workerCount))
				TermPrintnl("Client intensity: " + strconv.Itoa(clientIntensity) + "ms")
				strBuf := "Success: " + strconv.Itoa(success5s)
				strBuf += ", fail: " + strconv.Itoa(failed5s)
				if (failed5s + success5s) > 0{
					strBuf += ", rate: " + strconv.FormatFloat(100.0 * float64(success5s)/float64(failed5s + success5s), 'f', 2, 64) + "%"
				} else {
					strBuf += ", rate: 100.00%"
				}
				
				
				TermPrintnl(strBuf)
				TermPrintnl("----------------------------")
				
				success5s = 0
				failed5s = 0
				
				succMutex.Unlock()
				failMutex.Unlock()
			case <-statTimer:
				failMutex.Lock()
				succMutex.Lock()
				activeWorkersMutex.Lock()
				
				if (success + failed) > 0 {
					failRate := float64(failed) / float64(success + failed)
					if failRate > 0.2{
						workerCount += 1
						go worker(dataChanel, stopWorkerChan)
						TermPrintnl("Worker added: " + strconv.Itoa(workerCount))
					}
				}
				
				
				workerRate := float64(activeWorkers) / float64(workerCount)
				if workerRate <= 0.5 && workerCount > 0 {
					workerCount -= 1
					stopWorkerChan <- false
					TermPrintnl("Worker deleted: " + strconv.Itoa(workerCount))
				}
				
				success = 0
				failed = 0
				activeWorkers = 0
				
				activeWorkersMutex.Unlock()
				succMutex.Unlock()
				failMutex.Unlock()
			case ev:= <-keyEvent:
				switch(ev) {
					case '-':
						if clientCount > 0{
							clientCount -= 1
							stopClientChan <- false
							TermPrintnl("Client deleted: " + strconv.Itoa(clientCount))
						}
					case '+':
						clientCount += 1
						go client(dataChanel, stopClientChan)
						TermPrintnl("Client added: " + strconv.Itoa(clientCount))
					case '<':
						newIntent := int(float64(clientIntensity) * 0.9)
						if newIntent > 0 {
							clientIntensity = newIntent
							TermPrintnl("Intensity changed: " + strconv.Itoa(clientIntensity))
						}
					case '>':
						newIntent := int(float64(clientIntensity) * 1.1)
						clientIntensity = newIntent
						TermPrintnl("Intensity changed: " + strconv.Itoa(clientIntensity))
					case rune(17): //Ctrl + Q (to exit)
						qiut <- false
				}
		}
	}
}

func main() {
	
	rand.Seed(time.Now().UnixNano())
	
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
        panic(err)
	}
	defer terminal.Restore(int(os.Stdin.Fd()), oldState)
	term = terminal.NewTerminal(os.Stdin, "")
	
	exitFlag := make(chan bool)
	keyEvent := make(chan rune)
	
	clb := func(s string, i int, r rune) (string, int, bool) {
		if (r == '+' || r == '-' || r == '<' || r == '>' || r == 17){
			keyEvent <- r
		}
		return s, i, true
	}
	term.AutoCompleteCallback = clb
	
	go control(exitFlag, keyEvent)
	
	for{
		select{
			case <-exitFlag:
				break
			default:
				term.ReadLine()
		}
	}

}

