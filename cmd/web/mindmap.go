package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type MindMap struct {
	Id        string    `json:"id"`
	UserId    int       `json:"userId"`
	Content   *string   `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Name      *string   `json:"name"`
}

type MindFile struct {
	Name    *string `json:"name"`
	Content *string `json:"content"`
}

type MindMapEvent struct {
	Type      *string   `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func getMindMapHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webId := vars["id"]

	id := getIdFromWebId(webId)

	query := "select user_id, timestamp, name from mindmaps_meta where id=$1 and deleted=false"
	rows, err := db.Query(query, id)
	if err != nil {
		//log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if rows.Next() {
		mindmap := MindMap{Id: webId}
		err := rows.Scan(&mindmap.UserId, &mindmap.Timestamp, &mindmap.Name)
		if err != nil {
			//log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jsonMindMap, err := json.Marshal(mindmap)
		if err != nil {
			//log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(jsonMindMap))
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func createMindMapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	s, err := auth(w, r)
	if err != nil {
		return
	}

	var mf MindFile
	err = json.NewDecoder(r.Body).Decode(&mf)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	date := time.Now().UTC()

	query := "insert into mindmaps_meta (user_id, name, timestamp) values ($1, $2, $3) RETURNING id"
	rows, err := db.Query(query, s.Id, mf.Name, date)
	if err != nil {
		log.Println("createMindMapHandler", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	if rows.Next() {
		mindmap := MindMap{
			UserId:    s.Id,
			Timestamp: date,
		}
		var id int
		err = rows.Scan(&id)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		mindmap.Id = getWebId(id)

		jsonMindMap, err := json.Marshal(mindmap)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Location", "/api/mindmap/"+fmt.Sprint(mindmap.Id))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, string(jsonMindMap))
		return
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func removeMindMapHandler(w http.ResponseWriter, r *http.Request) {
	s, err := auth(w, r)
	if err != nil {
		return
	}

	vars := mux.Vars(r)
	webId := vars["id"]

	id := getIdFromWebId(webId)
	date := time.Now().UTC()

	query := "update mindmaps_meta set timestamp=$1, deleted=true where id=$2 and user_id=$3"
	rows, err := db.Query(query, date, id, s.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	w.WriteHeader(http.StatusOK)

	if file, ok := files[webId]; ok {
		msg := serviceMessage{
			Type: "delete",
			Data: MindMap{
				Id:        webId,
				Timestamp: date,
			},
		}
		file.broadcast(msg, "")
		delete(files, webId)
	}
}

func getMindMapsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
	s, err := auth(w, r)
	if err != nil {
		//log.Println(err)
		return
	}

	query := `select id, timestamp, name from mindmaps_meta where user_id=$1 and deleted=false 
	order by timestamp desc`

	rows, err := db.Query(query, s.Id)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	mindMapsCollection := []MindMap{}

	for rows.Next() {
		var id int
		mindmap := MindMap{}
		err := rows.Scan(&id, &mindmap.Timestamp, &mindmap.Name)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		mindmap.Id = getWebId(id)
		mindMapsCollection = append(mindMapsCollection, mindmap)
	}

	jsonMindMaps, err := json.Marshal(mindMapsCollection)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(jsonMindMaps))
}

func getMindMapEventsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: check params
	vars := mux.Vars(r)
	webId := vars["id"]
	min := r.URL.Query().Get("min")

	id := getIdFromWebId(webId)

	args := []any{id}
	query := "select type, content, timestamp from mindmaps_events where mindmap_id=$1 "
	if min != "" {
		query += "and timestamp>$2 "
		args = append(args, min)
	}
	query += "order by timestamp ASC"

	rows, err := db.Query(query, args...)

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	mindMapEventsCollection := []MindMapEvent{}

	for rows.Next() {
		mindmapEvent := MindMapEvent{}
		err := rows.Scan(&mindmapEvent.Type, &mindmapEvent.Content, &mindmapEvent.Timestamp)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mindMapEventsCollection = append(mindMapEventsCollection, mindmapEvent)
	}

	jsonMindMapEvents, err := json.Marshal(mindMapEventsCollection)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(jsonMindMapEvents))
}

func createMindMapEvent(webId string, eventType string, content string, date time.Time, name *string) error {
	mindmapId := getIdFromWebId(webId)

	query := `insert into mindmaps_events (mindmap_id, type, content, timestamp) 
	values ($1, $2, $3, $4)`

	rows, err := db.Query(query, mindmapId, eventType, content, date)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	if *name == "" {
		name = nil
	}

	// TODO: broker

	query = `update mindmaps_meta set timestamp=$1, name=COALESCE($2, name) 
	where id=$3 and deleted=false`

	rows, err = db.Query(query, date, name, mindmapId)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	return nil
}
