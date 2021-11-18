// Copyright 2019-present Facebook
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"path"
	"os"
	"log"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	log.SetFlags(log.Llongfile | log.LstdFlags)
	var (
		schemaPath = flag.String("path", "", "path to schema directory")
		targetPath = flag.String("targetPath", "", "target path")
	)
	flag.Parse()
	if *schemaPath == "" {
		log.Fatal("entproto: must specify schema path. use entproto -path ./ent/schema")
	}
	workingDir,err := os.Getwd()
	if err != nil{
		panic(err)
	}
	cfg :=&gen.Config{}
	log.Println("targetPath ", targetPath)
	if len(*targetPath) > 0{
		cfg.Target = path.Join(workingDir,*targetPath)
	}
	log.Println("tzzTarget ",cfg.Target)
	graph, err := entc.LoadGraph(*schemaPath, cfg)
	if err != nil {
		log.Fatalf("entproto: failed loading ent graph: %v", err)
	}
	if err := entproto.Generate(graph); err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}
