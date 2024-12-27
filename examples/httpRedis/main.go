// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client

func main() {
	initRedis()

	http.HandleFunc("/set", setHandler)
	http.HandleFunc("/get", getHandler)
	http.HandleFunc("/setex", setexHandler)
	http.HandleFunc("/sadd", saddHandler)
	http.HandleFunc("/pipeline", pipelineHandler)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initRedis() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	if redisHost == "" || redisPort == "" {
		redisAddr = "localhost:6379"
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err := rdb.Set(r.Context(), req.Key, req.Value, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Key-Value pair set successfully\n"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	value, err := rdb.Get(r.Context(), req.Key).Result()
	if err == redis.Nil {
		http.Error(w, "Key does not exist", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"key":   req.Key,
		"value": value,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func setexHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := rdb.SetEX(r.Context(), req.Key, req.Value, time.Second).Result()
	if err == redis.Nil {
		http.Error(w, "Key does not exist", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"key":   req.Key,
		"value": result,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func saddHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string   `json:"key"`
		Values []string `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err := rdb.SAdd(r.Context(), req.Key, req.Values).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"key":    req.Key,
		"values": req.Values,
		"status": "added successfully",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func pipelineHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Commands []struct {
			Command string `json:"command"`
			Key     string `json:"key"`
			Value   string `json:"value"`
		} `json:"commands"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pipe := rdb.Pipeline()

	var results []redis.Cmder
	for _, cmd := range req.Commands {
		switch cmd.Command {
		case "set":
			results = append(results, pipe.Set(r.Context(), cmd.Key, cmd.Value, 2))
		case "get":
			results = append(results, pipe.Get(r.Context(), cmd.Key))
		case "sadd":
			results = append(results, pipe.SAdd(r.Context(), cmd.Key, cmd.Value))
		default:
			http.Error(w, "Unsupported command", http.StatusBadRequest)
			return
		}
	}

	_, err := pipe.Exec(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var response []map[string]interface{}
	for _, result := range results {
		if result.Err() != nil {
			response = append(response, map[string]interface{}{
				"command": result.Name(),
				"error":   result.Err().Error(),
			})
		} else {
			switch cmd := result.(type) {
			case *redis.StatusCmd:
				response = append(response, map[string]interface{}{
					"command": result.Name(),
					"result":  cmd.Val(),
				})
			case *redis.StringCmd:
				response = append(response, map[string]interface{}{
					"command": result.Name(),
					"result":  cmd.Val(),
				})
			case *redis.IntCmd:
				response = append(response, map[string]interface{}{
					"command": result.Name(),
					"result":  cmd.Val(),
				})
			default:
				response = append(response, map[string]interface{}{
					"command": result.Name(),
					"result":  "unknown result type",
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Failed to encode response: %v", err)
		return
	}
}
