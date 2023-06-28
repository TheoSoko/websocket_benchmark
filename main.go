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

	"github.com/gorilla/websocket"
)

var (
	// receiver id 7
	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcGkuemVtdXMuaW5mbyIsImF1ZCI6ImFwaS56ZW11cy5pbmZvIiwic3ViIjoiNyIsInVzZXJFbWFpbCI6ImplYW5AZHVqYXJkaW4uZnIiLCJpYXQiOjE2ODc5NTQ2MTd9.SJkmSk84ANBaEuAs4YuRMd3mjsckoaTlyvVfy2RYWVY"
	// sender id 20
	tokenSender = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcGkuemVtdXMuaW5mbyIsImF1ZCI6ImFwaS56ZW11cy5pbmZvIiwic3ViIjoiMjAiLCJ1c2VyRW1haWwiOiJtaWNobWljaEBjYXJhbWFpbC5mciIsImlhdCI6MTY4Nzk1NjIwN30.BD_ofEWAW6WDehYcuv_VOL8rxe4pwt1VC8Soq82eC94"

	addr = flag.String("addr", "api.zemus.info:4000", "http service address")

	connList = make(map[int]*websocket.Conn)

	latencies = make([]int, 0)

	done = make(chan bool)
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	defer abruptInterrupt()

	defer close(done)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}

	// CREATING THE WS CONNECTIONS
	for i := 0; i < 1; i++ {
		log.Printf("connecting to %s, for the %dth time", u.String(), i+1)

		conn, _, err := websocket.DefaultDialer.Dial(u.String()+"?id=7", http.Header{"Authorization": {"bearer " + token}})
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer conn.Close()

		connList[i] = conn

		go func() {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					log.Println("Erreur à la lecture d'un message, socket probablement fermée:", err)
					return
				}
				//log.Printf("reçu: %s", message)
				splitMess := strings.Split(string(message), " ")
				lastItem := strings.Trim(splitMess[len(splitMess)-1], "\n")
				sentTime, err := strconv.Atoi(lastItem)
				if err != nil {
					fmt.Println("ERREUR, mauvaise conversion en int à sentTime")
				}

				receivedTime := time.Now().UnixMilli()
				difference := int(receivedTime) - sentTime

				fmt.Println("sentTime == ", sentTime)
				fmt.Println("ReceivedTime == ", receivedTime)
				fmt.Println("Difference == ", difference, " milliseconds")

				latencies = append(latencies, difference)
			}
		}()
	}

	// TICKS EVERY SECOND
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			// if OS interruption
			case <-interrupt:
				log.Println("interruption asked")
				abruptInterrupt()
				return

			// EVERY SECOND, SENDS MESSAGE
			case t := <-ticker.C:
				stamp := fmt.Sprint(t.UnixMilli()) // Timestamp in milliseconds
				postJson := bytes.NewBuffer([]byte(`{"message":"` + stamp + `"}`))

				request, err := http.NewRequest("POST", "http://api.zemus.info:4000/users/20/friends/7/message", postJson)
				if err != nil {
					panic("Panique à connexion http, \n " + err.Error())
				}
				request.Header.Add("Authorization", "Bearer "+tokenSender)

				client := http.Client{Timeout: time.Duration(5) * time.Second}
				resp, err := client.Do(request)
				if err != nil {
					panic("Panique à client.Do, \n " + err.Error())
				}
				if resp.StatusCode != 204 {
					log.Println("ERREUR, le status de la réponse n'est pas 204")
				}

			// if n seconds passed
			case <-done:
				return
			}
		}
	}()

	time.Sleep(5 * time.Second)
	ticker.Stop()
	done <- true

	averageLatency()

}

// In case the program is shut down
func abruptInterrupt() {
	for i2, c := range connList {
		c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		<-time.After(time.Second)
		delete(connList, i2)
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
