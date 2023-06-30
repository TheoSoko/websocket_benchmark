package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"sync"

	"github.com/gorilla/websocket"
)

var (
	// receiver id 7
	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcGkuemVtdXMuaW5mbyIsImF1ZCI6ImFwaS56ZW11cy5pbmZvIiwic3ViIjoiNyIsInVzZXJFbWFpbCI6ImplYW5AZHVqYXJkaW4uZnIiLCJpYXQiOjE2ODc5NTQ2MTd9.SJkmSk84ANBaEuAs4YuRMd3mjsckoaTlyvVfy2RYWVY"
	// sender id 20
	tokenSender = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcGkuemVtdXMuaW5mbyIsImF1ZCI6ImFwaS56ZW11cy5pbmZvIiwic3ViIjoiMjAiLCJ1c2VyRW1haWwiOiJtaWNobWljaEBjYXJhbWFpbC5mciIsImlhdCI6MTY4Nzk1NjIwN30.BD_ofEWAW6WDehYcuv_VOL8rxe4pwt1VC8Soq82eC94"

	addrListen = flag.String("addrListen", "127.0.0.1:4000", "websocket address")
	addrSend   = flag.String("addrSend", "127.0.0.1:4000", "http address for sending messages")

	connList = make(map[int]**websocket.Conn)

	latencies = make([]int, 0)

	done = make(chan bool)

	mutex sync.Mutex
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	urlListen := url.URL{Scheme: "ws", Host: *addrListen, Path: "/ws", RawQuery: "id=7"}
	urlSend := url.URL{Scheme: "http", Host: *addrSend, Path: "/users/20/friends/7/message"}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	defer interruptConn()

	defer close(done)

	// CREATING THE WS CONNECTIONS
	for i := 0; i < 1000; i++ {
		log.Printf("connecting to %s, for the %dth time", urlListen.String(), i+1)

		conn, httpRes, err := websocket.DefaultDialer.Dial(urlListen.String(), http.Header{"Authorization": {"bearer " + token}})
		if err != nil {
			if httpRes != nil {
				log.Println("http response code", httpRes.StatusCode)
			}
			log.Fatal("dial:", err)
		}
		defer func() {
			mutex.Lock()
			conn.Close()
			mutex.Unlock()
		}()

		connList[i] = &conn

		go func() {
			for {
				msType, message, err := conn.ReadMessage()
				if msType == -1 {
					fmt.Println("Message type -1 (fermeture de websocket).")
					return
				}
				if err != nil {
					log.Println("Erreur à la lecture d'un message:", err)
					return
				}
				//log.Printf("reçu: %s", message)
				splt := strings.Split(string(message), " ")
				lastItem := strings.Trim(splt[len(splt)-1], "\n")
				sentTime, err := strconv.Atoi(lastItem)
				if err != nil {
					fmt.Println("ERREUR, mauvaise conversion en int à sentTime")
					return
				}

				receivedTime := time.Now().UnixMilli()
				difference := int(receivedTime) - sentTime

				fmt.Println("Difference == ", difference, " milliseconds")

				latencies = append(latencies, difference)
			}
		}()
	}

	// TICKS EVERY SECOND
	ticker := time.NewTicker(time.Microsecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			// if OS interruption
			case <-interrupt:
				log.Println("interruption asked")
				interruptConn()
				return
			// EVERY SECOND, SENDS MESSAGE
			case t := <-ticker.C:
				go sendMessage(t, urlSend)
			// if n seconds passed
			case <-done:
				interruptConn()
				return
			}
		}
	}()

	time.Sleep(20 * time.Second)
	ticker.Stop()
	done <- true
	time.Sleep(5 * time.Second)

	averageLatency()

}

// In case the program is shut down
func interruptConn() {
	for i2, c := range connList {
		conn := *c
		mutex.Lock()
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		mutex.Unlock()
		<-time.After(time.Second)
		mutex.Lock()
			delete(connList, i2)
		mutex.Unlock()
	}
}

func averageLatency() {
	sum := 0
	for _, l := range latencies {
		sum += l
	}
	avg := sum / len(latencies)

	fmt.Println("Average latency is", avg, "milliseconds")
}

func sendMessage(t time.Time, url url.URL){
	stamp := fmt.Sprint(t.UnixMilli()) // Timestamp in milliseconds
	postJson := bytes.NewBuffer([]byte(`{"message":"` + stamp + `"}`))

	request, err := http.NewRequest("POST", url.String(), postJson)
	if err != nil {
		panic("Panique à connexion http, \n " + err.Error())
	}
	request.Header.Add("Authorization", "Bearer "+tokenSender)

	client := http.Client{Timeout: time.Duration(5) * time.Second}
	resp, err := client.Do(request)

	fmt.Println("sent")

	if err != nil {
		panic("Panique à client.Do, \n " + err.Error())
	}
	if resp.StatusCode != 204 {
		log.Println("ERREUR, le status de la réponse n'est pas 204, mais", resp.StatusCode)
	}

}