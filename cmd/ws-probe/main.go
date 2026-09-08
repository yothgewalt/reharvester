package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func main() {
	action := flag.String("action", "", "run a project action instead of a harvest (add|subtract|extract|summarize)")
	project := flag.String("project", "dev", "project id for --action")
	flag.Parse()

	url := "http://localhost:8000/api/v1/harvest/init"
	var body *strings.Reader = strings.NewReader(`{"keywords":["neural ranking","query expansion","learning to rank","passage retrieval","re-ranking"]}`)
	if *action != "" {
		url = "http://localhost:8000/api/v1/projects/" + *project + "/action"
		body = strings.NewReader(`{"action":"` + *action + `"}`)
	}
	res, err := http.Post(url, "application/json", body)
	if err != nil {
		log.Fatal(err)
	}
	var init struct {
		TaskID     string `json:"taskId"`
		StreamPath string `json:"streamPath"`
	}
	json.NewDecoder(res.Body).Decode(&init)
	res.Body.Close()
	fmt.Printf("taskId=%s streamPath=%s\n", init.TaskID, init.StreamPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws://localhost:8000/api/v1/harvest/stream/"+init.TaskID,
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://localhost:3000"}}})
	if err != nil {
		log.Fatal(err)
	}
	// Browsers impose no frame-size limit; raise the probe's so it measures the
	// server rather than its own default.
	c.SetReadLimit(8 << 20)
	defer c.CloseNow()

	counts := map[string]int{}
	nodes := map[string]bool{}
	badEdges, lastPct := 0, -1
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			fmt.Printf("socket closed: %v\n", err)
			break
		}
		var probe struct {
			Type       string                            `json:"type"`
			Percent    int                               `json:"percent"`
			Stage      string                            `json:"stage"`
			Entry      struct{ Level, Message string }   `json:"entry"`
			AddedNodes []struct{ ID string }             `json:"addedNodes"`
			AddedEdges []struct{ Source, Target string } `json:"addedEdges"`
			Summary    struct {
				DocsIngested int `json:"docsIngested"`
				DurationMs   int `json:"durationMs"`
			} `json:"summary"`
		}
		if json.Unmarshal(data, &probe) != nil {
			fmt.Println("UNPARSEABLE FRAME — the client would drop this silently")
			continue
		}
		counts[probe.Type]++
		switch probe.Type {
		case "log":
			fmt.Printf("  [%s] %s\n", probe.Entry.Level, probe.Entry.Message)
		case "progress":
			if probe.Percent != lastPct {
				fmt.Printf("  %3d%% %s\n", probe.Percent, probe.Stage)
				lastPct = probe.Percent
			}
		case "graph_delta":
			for _, n := range probe.AddedNodes {
				nodes[n.ID] = true
			}
			for _, e := range probe.AddedEdges {
				if !nodes[e.Source] || !nodes[e.Target] {
					badEdges++
				}
			}
		case "done":
			fmt.Printf("DONE docsIngested=%d durationMs=%d\n", probe.Summary.DocsIngested, probe.Summary.DurationMs)
		}
	}
	fmt.Printf("frames: %v\nnodes streamed: %d, edges referencing unknown nodes: %d\n", counts, len(nodes), badEdges)
	if counts["done"] == 0 {
		fmt.Println("FAIL: socket closed without a done frame — the UI would mark this project failed")
		os.Exit(1)
	}
	if badEdges > 0 {
		fmt.Println("FAIL: edges arrived before their endpoints")
		os.Exit(1)
	}
	fmt.Println("PASS")
}
