// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/redis/rueidis"
)

const testKey = "test_key"

type server struct {
	client rueidis.Client
}

func (s *server) Do(w http.ResponseWriter, req *http.Request) {
	randomValue := fmt.Sprintf("random_value_%d", rand.Intn(1000))
	err := setKey(req.Context(), s.client, testKey, randomValue)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to set key: %v", err))
	}
	fmt.Printf("Set key '%s' with value: %s\n", testKey, randomValue)

	value, err := getKey(req.Context(), s.client, testKey)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to get key: %v", err))
	}
	fmt.Printf("Got value for key '%s': %s\n", testKey, value)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(map[string]string{"value": value})
}

func main() {
	address := "redis:6379"
	fmt.Println("Connecting to redis server...")
	fmt.Println(address)

	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{address},
		SelectDB:    0,
	})
	if err != nil {
		log.Fatalf("Failed to create Redis client: %v", err)
	}
	defer client.Close()

	s := &server{client: client}

	http.HandleFunc("/do", s.Do)
	port := ":8081"
	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func setKey(ctx context.Context, client rueidis.Client, key, value string) error {
	cmd := client.B().Set().Key(key).Value(value).Build()
	return client.Do(ctx, cmd).Error()
}

func getKey(ctx context.Context, client rueidis.Client, key string) (string, error) {
	cmd := client.B().Get().Key(key).Build()
	result := client.Do(ctx, cmd)
	if result.Error() != nil {
		return "", result.Error()
	}
	return result.ToString()
}
