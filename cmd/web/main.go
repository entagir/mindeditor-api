package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"database/sql"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"

	"github.com/gorilla/mux"

	"github.com/speps/go-hashids/v2"
)

type Config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Db struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Dbname   string `json:"dbname"`
	} `json:"db"`
	Redis struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Password string `json:"password"`
		Db       int    `json:"db"`
	} `json:"redis"`
	Hashids struct {
		Salt      string `json:"salt"`
		Minlength int    `json:"minlength"`
	} `json:"hashids"`
}

var config Config

var ctx context.Context
var db *sql.DB
var rdb *redis.Client

var h *hashids.HashID

func initDB(c *Config) {
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
		c.Db.User, c.Db.Password, c.Db.Dbname)

	pg, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	db = pg

	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port),
		Password: c.Redis.Password,
		DB:       c.Redis.Db,
	})

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx = context.Background()

	f, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		panic(err)
	}

	initDB(&config)

	hd := hashids.NewData()
	hd.Salt = config.Hashids.Salt
	hd.MinLength = config.Hashids.Minlength
	h, err = hashids.NewWithData(hd)
	if err != nil {
		panic(err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/api/signin", Signin).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/signup", Signup).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/logout", Logout).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/ping", Ping).Methods("POST", "OPTIONS")

	router.HandleFunc("/api/mindmap", getMindMapsList).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/mindmap", createMindMapHandler).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/mindmap/{id}", getMindMapHandler).Methods("GET")
	router.HandleFunc("/api/mindmap/{id}", removeMindMapHandler).Methods("DELETE")

	router.HandleFunc("/api/mindmap/events/{id}", getMindMapEventsHandler).Methods("GET")

	http.HandleFunc("/api/ws", wsEndpoint)

	http.Handle("/", router)

	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	fmt.Println("[", addr, "]", "Server is listening", "...")
	log.Fatal(http.ListenAndServe(addr, nil))
}
