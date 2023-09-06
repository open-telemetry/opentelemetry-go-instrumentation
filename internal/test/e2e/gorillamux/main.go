// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/users/{user}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		user := vars["user"]
		fmt.Fprintf(w, "Hello user %s\n", user)
	})
	go func() {
		_ = http.ListenAndServe(":8080", r)
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/users/foo")
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Body: %s\n", string(body))
	_ = resp.Body.Close()

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
