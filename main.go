package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/flowee-ru/monday/utils"
	"github.com/joho/godotenv"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/av/pubsub"
	"github.com/nareix/joy4/format"
	"github.com/nareix/joy4/format/flv"
	"github.com/nareix/joy4/format/rtmp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var ctx = context.TODO()

type writeFlusher struct {
	httpflusher http.Flusher
	io.Writer
}

func (f writeFlusher) Flush() error {
	f.httpflusher.Flush()
	return nil
}

func init() {
	format.RegisterAll()
}

func main() {
	godotenv.Load()

	wsPort := "8089"
	if os.Getenv("WEBSERVER_PORT") != "" {
		wsPort = os.Getenv("WEBSERVER_PORT")
	}

	rtmpPort := "1935"
	if os.Getenv("RTMP_PORT") != "" {
		rtmpPort = os.Getenv("RTMP_PORT")
	}

	db, err := utils.ConnectMongo(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	server := &rtmp.Server{
		Addr: ":" + rtmpPort,
	}

	l := &sync.RWMutex{}
	type Channel struct {
		que *pubsub.Queue
	}
	channels := map[string]*Channel{}

	server.HandlePlay = func(conn *rtmp.Conn) {
		l.RLock()
		ch := channels[conn.URL.Path]
		l.RUnlock()

		if ch != nil {
			cursor := ch.que.Latest()
			avutil.CopyFile(conn, cursor)
		}
	}

	server.HandlePublish = func(conn *rtmp.Conn) {
		streams, _ := conn.Streams()

		path := strings.Split(conn.URL.Path, "/")
		accountIDHex := path[len(path) - 1]
		token := conn.URL.Query().Get("t")
	
		if accountIDHex == "" || token == "" || !primitive.IsValidObjectID(accountIDHex) {
			conn.Close()
			return
		}

		accountID, _ := primitive.ObjectIDFromHex(accountIDHex)

		err := db.Collection("accounts").FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: accountID}, primitive.E{Key: "streamToken", Value: token}, primitive.E{Key: "isActive", Value: true}}).Decode(nil)
		if err == mongo.ErrNoDocuments {
			conn.Close()
			return
		}

		_, err = db.Collection("accounts").UpdateOne(ctx, bson.D{primitive.E{Key: "_id", Value: accountID}}, bson.D{primitive.E{Key: "$set", Value: bson.D{primitive.E{Key: "isLive", Value: true}}}})
		if err != nil {
			conn.Close()
			return
		}

		log.Println(accountIDHex + " is streaming")

		defer func() {
			_, err = db.Collection("accounts").UpdateOne(ctx, bson.D{primitive.E{Key: "_id", Value: accountID}}, bson.D{primitive.E{Key: "$set", Value: bson.D{primitive.E{Key: "isLive", Value: false}}}})
			if err != nil {
				conn.Close()
				return
			}

			log.Println(accountIDHex + " has finished his stream")
		}()

		l.Lock()
		ch := channels[conn.URL.Path]
		if ch == nil {
			ch = &Channel{}
			ch.que = pubsub.NewQueue()
			ch.que.WriteHeader(streams)
			channels[conn.URL.Path] = ch
		} else {
			ch = nil
		}
		l.Unlock()
		if ch == nil {
			return
		}

		avutil.CopyPackets(ch.que, conn)

		l.Lock()
		delete(channels, conn.URL.Path)
		l.Unlock()
		ch.que.Close()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		l.RLock()
		ch := channels[r.URL.Path]
		l.RUnlock()

		if ch != nil {
			w.Header().Set("Content-Type", "video/x-flv")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.WriteHeader(200)
			flusher := w.(http.Flusher)
			flusher.Flush()

			muxer := flv.NewMuxerWriteFlusher(writeFlusher{httpflusher: flusher, Writer: w})
			cursor := ch.que.Latest()

			avutil.CopyFile(muxer, cursor)
		} else {
			http.NotFound(w, r)
		}
	})

	log.Println("Starting web server...")
	go http.ListenAndServe(":" + wsPort, nil)

	log.Println("Starting RTMP server...")
	server.ListenAndServe()
}