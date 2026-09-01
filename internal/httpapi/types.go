// Package httpapi serves the local API the web UI talks to. Every type here
// mirrors web/src/types/domain.ts field for field; the frontend is already
// written against this contract, so the shapes are fixed.
package httpapi

// GraphNodeData carries what the UI needs to draw and filter a node.
type GraphNodeData struct {
	Kind       string  `json:"kind"` // "paper" | "concept"
	DocID      string  `json:"docId"`
	Year       int     `json:"year"`
	PageRank   float64 `json:"pageRank"`
	Gap        bool    `json:"gap"`
	Bridge     float64 `json:"bridge"` // fraction of edge weight crossing a community, [0,1]
	Cluster    string  `json:"cluster"`
	Category   string  `json:"category"` // primary arXiv category, "" for concepts
	OpenAccess bool    `json:"openAccess"`
	Frequency  *int    `json:"frequency,omitempty"`
}

// Community is one Louvain partition as the reader's left rail needs it. Size
// counts every member; MemberDocIDs only lists those inside the snapshot, so a
// large community can report thousands of members and a handful of links.
// CommunityLink is one edge of the coarse-grained field map: how many backbone
// edges run between two communities. Source and Target are Community.ID.
type CommunityLink struct {
	Source int `json:"source"`
	Target int `json:"target"`
	Count  int `json:"count"`
}

type Community struct {
	ID           int      `json:"id"`
	Label        string   `json:"label"`
	Slug         string   `json:"slug"` // joins against GraphNodeData.Cluster
	Size         int      `json:"size"`
	Terms        []string `json:"terms"`
	MemberDocIDs []string `json:"memberDocIds"`
}

// GraphNode's ID must equal Data.DocID and must be the slug of Label. The UI
// resolves [[wikilinks]] by slugging the link text and matching node ids, and
// fetches wiki markdown by docId.
type GraphNode struct {
	ID    string        `json:"id"`
	Label string        `json:"label"`
	Data  GraphNodeData `json:"data"`
}

type GraphEdge struct {
	ID       string  `json:"id"`
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Weight   float64 `json:"weight"`
	Relation string  `json:"relation"`
}

type GraphSnapshot struct {
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	GeneratedAt string      `json:"generatedAt"`
}

// TrendsPerDirection is how many rising and how many declining terms a trends
// response carries, matching the paper's Fig. 2a.
const TrendsPerDirection = 7

type TrendKeyword struct {
	Keyword string `json:"keyword"`
	// Count is the late-window document count; EarlyDocs the early-window one.
	// Both ship because a lift alone cannot distinguish a genuinely new phrase
	// from a merely growing one, nor show a decline as "50 documents to 2".
	Count      int     `json:"count"`
	EarlyDocs  int     `json:"earlyDocs"`
	GrowthRate float64 `json:"growthRate"`
	// Direction splits the two halves of the diverging view; Rank is within it.
	Direction string `json:"direction"` // "rising" | "declining"
	Rank      int    `json:"rank"`
	// Specificity is the normalised entropy over communities that admitted the
	// term (<= 0.80); Burst is its Kleinberg weight, high when the rise is
	// concentrated in time rather than spread across the window.
	Specificity float64 `json:"specificity"`
	Burst       float64 `json:"burst"`
	// Trajectory is the term's share of documents per year across the whole
	// corpus, not just the selected window. Net growth hides substitution: a
	// phrase that peaked and collapsed and one still climbing can carry the same
	// lift, and only the year-by-year series tells them apart.
	Trajectory []TrendPoint `json:"trajectory,omitempty"`
}

type TrendPoint struct {
	Year       int     `json:"year"`
	Prevalence float64 `json:"prevalence"`
}

// HealthStatus is polled every 30 s while online and must answer within 3 s.
// Tier is an addition to the frontend's type: it surfaces which rung of the
// capability ladder is actually running.
type HealthStatus struct {
	Status  string `json:"status"`
	LLM     string `json:"llm"` // "ok" | "unreachable"
	Version string `json:"version"`
	Tier    string `json:"tier"`
}

// TaskResponse answers both harvest init and project actions. StreamPath is
// advisory: the client rebuilds the socket URL from TaskID itself.
type TaskResponse struct {
	TaskID     string `json:"taskId"`
	StreamPath string `json:"streamPath"`
}

type SchedulerProfile struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Keywords  []string `json:"keywords"`
	Cron      string   `json:"cron"`
	CreatedAt string   `json:"createdAt"`
	Status    string   `json:"status"`
}

type ProjectSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Query        string `json:"query"`
	CreatedAt    string `json:"createdAt"`
	Status       string `json:"status"`
	DocsIngested int    `json:"docsIngested"`
}

type GapPositions struct {
	GapNodeIDs []string `json:"gapNodeIds"`
}

type CrawlLogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // "info" | "warn" | "error" | "success"
	Message   string `json:"message"`
}

// The four crawl stream frames. The client discriminates on Type and silently
// drops anything it cannot parse, so these shapes must be exact.
type (
	logFrame struct {
		Type  string        `json:"type"`
		Entry CrawlLogEntry `json:"entry"`
	}
	progressFrame struct {
		Type    string `json:"type"`
		Percent int    `json:"percent"`
		Stage   string `json:"stage"`
	}
	deltaFrame struct {
		Type       string      `json:"type"`
		AddedNodes []GraphNode `json:"addedNodes"`
		AddedEdges []GraphEdge `json:"addedEdges"`
	}
	doneFrame struct {
		Type    string      `json:"type"`
		Summary doneSummary `json:"summary"`
	}
)

type doneSummary struct {
	DocsIngested int   `json:"docsIngested"`
	DurationMs   int64 `json:"durationMs"`
}
