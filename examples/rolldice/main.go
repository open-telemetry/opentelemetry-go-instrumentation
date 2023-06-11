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
	"math/rand"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func rolldice(w http.ResponseWriter, _ *http.Request) {
	n := rand.Intn(6)
	logger.Info("rolldice called", zap.Int("dice", n))
	fmt.Fprintf(w, "%v", n)
}

var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		fmt.Printf("error creating zap logger, error:%v", err)
		return
	}
	port := fmt.Sprintf(":%d", 8080)
	logger.Info("starting http server", zap.String("port", port))

	http.HandleFunc("/rolldice", rolldice)
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Error("error running server", zap.Error(err))
	}
}
