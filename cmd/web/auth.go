package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"database/sql"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type session struct {
	Id     int       `json:"id"`
	Login  string    `json:"login"`
	Expiry time.Time `json:"expiry"`
	Token  string    `json:"token"`
}

func (s session) isExpired() bool {
	return s.Expiry.Before(time.Now())
}

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func auth(w http.ResponseWriter, r *http.Request) (*session, error) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return nil, fmt.Errorf("options")
	}

	authHeader, exists := r.Header["Authorization"]
	if !exists || len(authHeader) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, fmt.Errorf("no token")
	}

	sessionToken := authHeader[0]

	val, err := rdb.Get(ctx, sessionToken).Result()
	if err != nil || val == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, fmt.Errorf("not exist session")
	}

	var userSession session
	err = json.Unmarshal([]byte(val), &userSession)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("not exist session")
	}

	if userSession.isExpired() {
		rdb.Del(ctx, sessionToken)
		w.WriteHeader(http.StatusUnauthorized)
		return nil, fmt.Errorf("session expired")
	}

	return &userSession, nil
}

func Signup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var c credentials
	err := json.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		//log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if c.Login == "" || c.Password == "" {
		//log.Println("No creds")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rows, err := db.Query("select * from users where login=$1", c.Login)
	if err != nil {
		//log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if rows.Next() {
		//log.Println("user exist")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(c.Password), 8)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	query := `insert into users (login, password, name, lastname) values ($1, $2, $3, $4) 
	RETURNING id`

	if rows, err = db.Query(query, c.Login, string(hashedPassword), nil, nil); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessionToken := uuid.NewString()
		timeDuration := time.Duration(24 * 180 * time.Hour)
		expiresAt := time.Now().Add(timeDuration)

		jsonSession, err := json.Marshal(session{
			Id:     id,
			Login:  c.Login,
			Expiry: expiresAt,
			Token:  sessionToken,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = rdb.Set(ctx, sessionToken, jsonSession, timeDuration).Err()
		if err != nil {
			fmt.Println(err)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fmt.Sprint("{\"id\":\"", id, "\", \"session\":\"", sessionToken, "\"}"))

		return
	} else {
		log.Println("no returning")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func Signin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var c credentials
	err := json.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	result := db.QueryRow("select id, password from users where login=$1", c.Login)

	var id int
	var storedPassword string
	err = result.Scan(&id, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(c.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	sessionToken := uuid.NewString()
	timeDuration := time.Duration(24 * 180 * time.Hour)
	expiresAt := time.Now().Add(timeDuration)

	jsonSession, err := json.Marshal(session{
		Id:     id,
		Login:  c.Login,
		Expiry: expiresAt,
		Token:  sessionToken,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = rdb.Set(ctx, sessionToken, jsonSession, timeDuration).Err()
	if err != nil {
		fmt.Println(err)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, fmt.Sprint("{\"id\":\"", id, "\", \"session\":\"", sessionToken, "\"}"))

	//log.Println(sessions)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	s, err := auth(w, r)
	if err != nil {
		return
	}

	rdb.Del(ctx, s.Token)
	//log.Println(sessions)
}

func Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	_, err := auth(w, r)
	if err != nil {
		return
	}

	w.WriteHeader(http.StatusOK)
}
