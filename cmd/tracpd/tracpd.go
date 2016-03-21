package main

import (
	"github.com/chzyer/flow"
	"github.com/chzyer/tracp"
	"gopkg.in/logex.v1"
)

func main() {
	flow.DefaultDebug = true
	cfg := tracp.NewConfig()
	f := flow.New()
	t := tracp.NewTracp(f, cfg)
	t.Run()
	if err := f.Wait(); err != nil {
		logex.Error(err)
	}
}
